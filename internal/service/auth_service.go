package service

import (
	"context"
	"errors"
	"log/slog"
	"regexp"
	"strings"
	"time"

	"github.com/deepsi/agent-service/internal/config"
	"github.com/deepsi/agent-service/internal/domain"
	"github.com/deepsi/agent-service/internal/platform/security"
	"github.com/google/uuid"
)

var emailPattern = regexp.MustCompile(`^[^\s@]+@[^\s@]+\.[^\s@]+$`)

type RegisterInput struct {
	Name     string
	Email    string
	Password string
}

type AuthService struct {
	cfg      *config.Config
	users    domain.UserRepository
	tokens   domain.TokenRepository
	sessions *SessionService
	email    *EmailService
	log      *slog.Logger
}

func NewAuthService(
	cfg *config.Config,
	users domain.UserRepository,
	tokens domain.TokenRepository,
	sessions *SessionService,
	email *EmailService,
	log *slog.Logger,
) *AuthService {
	return &AuthService{cfg: cfg, users: users, tokens: tokens, sessions: sessions, email: email, log: log}
}

func (s *AuthService) Register(ctx context.Context, input RegisterInput, meta SessionMeta) (*AuthResult, error) {
	name := strings.TrimSpace(input.Name)
	email := normalizeEmail(input.Email)
	if len([]rune(name)) < 2 || len([]rune(name)) > 80 {
		return nil, domain.Invalid("invalid_name", "Tên cần từ 2 đến 80 ký tự.")
	}
	if !emailPattern.MatchString(email) {
		return nil, domain.Invalid("invalid_email", "Email chưa đúng định dạng.")
	}
	if err := security.ValidatePassword(input.Password); err != nil {
		return nil, domain.Invalid("weak_password", err.Error())
	}
	if existing, _ := s.users.FindByEmail(ctx, email); existing != nil {
		return nil, domain.Conflict("email_exists", "Email này đã có tài khoản. Hãy đăng nhập hoặc đặt lại mật khẩu.")
	}

	hash, err := security.HashPassword(input.Password, s.cfg.Auth.BcryptCost)
	if err != nil {
		return nil, err
	}
	count, _ := s.users.Count(ctx)
	user := &domain.User{
		ID:            "usr_" + uuid.NewString(),
		Name:          name,
		Email:         email,
		PasswordHash:  hash,
		Provider:      domain.ProviderPassword,
		Role:          domain.RoleClient,
		Plan:          domain.PlanFree,
		EmailVerified: !s.cfg.Auth.RequireEmailVerification,
		AvatarColor:   security.AvatarColor(int(count)),
	}
	if err := s.users.Create(ctx, user); err != nil {
		if errors.Is(err, domain.ErrDuplicate) {
			return nil, domain.Conflict("email_exists", "Email này đã có tài khoản. Hãy đăng nhập hoặc đặt lại mật khẩu.")
		}
		return nil, err
	}

	if s.cfg.Auth.RequireEmailVerification {
		if err := s.sendVerification(ctx, user); err != nil {
			s.log.Error("send verification email failed", "user_id", user.ID, "error", err)
		}
		return &AuthResult{
			User:                      user.Public(),
			RequiresEmailVerification: true,
			Message:                   "Tài khoản đã được tạo. Kiểm tra email để xác nhận trước khi đăng nhập.",
		}, nil
	}

	result, err := s.sessions.Issue(ctx, user, meta)
	if err != nil {
		return nil, err
	}
	go s.sendWelcome(user)
	return result, nil
}

func (s *AuthService) Login(ctx context.Context, email, password string, meta SessionMeta) (*AuthResult, error) {
	user, err := s.users.FindByEmail(ctx, normalizeEmail(email))
	if err != nil || user.Provider != domain.ProviderPassword || user.PasswordHash == "" {
		return nil, domain.Unauthorized("invalid_credentials", "Email hoặc mật khẩu không đúng.")
	}
	if !security.CheckPassword(user.PasswordHash, password) {
		return nil, domain.Unauthorized("invalid_credentials", "Email hoặc mật khẩu không đúng.")
	}
	if s.cfg.Auth.RequireEmailVerification && !user.EmailVerified {
		return nil, domain.Forbidden("email_not_verified", "Email chưa được xác nhận. Kiểm tra hộp thư hoặc gửi lại email xác nhận.")
	}
	_ = s.users.TouchLogin(ctx, user.ID, time.Now())
	result, err := s.sessions.Issue(ctx, user, meta)
	if err != nil {
		return nil, err
	}
	return result, nil
}

func (s *AuthService) Refresh(ctx context.Context, rawRefresh string, meta SessionMeta) (*AuthResult, error) {
	if rawRefresh == "" {
		return nil, domain.Unauthorized("missing_refresh_token", "Thiếu phiên đăng nhập.")
	}
	return s.sessions.Rotate(ctx, rawRefresh, meta)
}

func (s *AuthService) Logout(ctx context.Context, rawRefresh string) error {
	return s.sessions.Revoke(ctx, rawRefresh)
}

func (s *AuthService) Me(ctx context.Context, userID string) (*domain.User, error) {
	user, err := s.users.FindByID(ctx, userID)
	if err != nil {
		return nil, domain.Unauthorized("user_not_found", "Tài khoản không tồn tại.")
	}
	if user.Plan != domain.PlanFree && user.PlanExpiresAt != nil && user.PlanExpiresAt.Before(time.Now()) {
		_ = s.users.SetPlan(ctx, user.ID, domain.PlanFree, nil, nil, false)
		user.Plan = domain.PlanFree
		user.PlanExpiresAt = nil
		user.CancelAtPeriodEnd = false
	}
	return user.Public(), nil
}

func (s *AuthService) UpdateProfile(ctx context.Context, userID, name, avatarColor string) (*domain.User, error) {
	name = strings.TrimSpace(name)
	if name != "" && (len([]rune(name)) < 2 || len([]rune(name)) > 80) {
		return nil, domain.Invalid("invalid_name", "Tên cần từ 2 đến 80 ký tự.")
	}
	if avatarColor != "" {
		allowed := map[string]bool{"moss": true, "clay": true, "ochre": true, "blue": true, "lavender": true}
		if !allowed[avatarColor] {
			return nil, domain.Invalid("invalid_avatar_color", "Màu đại diện không hợp lệ.")
		}
	}
	if err := s.users.UpdateProfile(ctx, userID, name, avatarColor); err != nil {
		return nil, err
	}
	return s.Me(ctx, userID)
}

func (s *AuthService) ChangePassword(ctx context.Context, userID, current, next string, meta SessionMeta) (*AuthResult, error) {
	if err := security.ValidatePassword(next); err != nil {
		return nil, domain.Invalid("weak_password", err.Error())
	}
	user, err := s.users.FindByID(ctx, userID)
	if err != nil {
		return nil, domain.Unauthorized("user_not_found", "Tài khoản không tồn tại.")
	}
	if user.Provider == domain.ProviderPassword && !security.CheckPassword(user.PasswordHash, current) {
		return nil, domain.Unauthorized("invalid_password", "Mật khẩu hiện tại không đúng.")
	}
	hash, err := security.HashPassword(next, s.cfg.Auth.BcryptCost)
	if err != nil {
		return nil, err
	}
	if err := s.users.SetPasswordHash(ctx, userID, hash); err != nil {
		return nil, err
	}
	if err := s.sessions.RevokeAll(ctx, userID); err != nil {
		return nil, err
	}
	return s.sessions.Issue(ctx, user, meta)
}

func (s *AuthService) ForgotPassword(ctx context.Context, email string) (string, error) {
	email = normalizeEmail(email)
	if !emailPattern.MatchString(email) {
		return "", domain.Invalid("invalid_email", "Email chưa đúng định dạng.")
	}
	user, err := s.users.FindByEmail(ctx, email)
	if err != nil {
		// Do not reveal whether an account exists.
		return "", nil
	}
	raw, err := security.RandomToken(32)
	if err != nil {
		return "", err
	}
	token := &domain.OneTimeToken{
		ID:        "ot_" + uuid.NewString(),
		UserID:    user.ID,
		Email:     user.Email,
		Purpose:   domain.TokenPurposeResetPassword,
		TokenHash: security.HashToken(raw),
		ExpiresAt: time.Now().Add(s.cfg.Auth.ResetPasswordTTL),
		CreatedAt: time.Now(),
	}
	if err := s.tokens.CreateOneTime(ctx, token); err != nil {
		return "", err
	}
	if err := s.email.SendResetPassword(ctx, user, raw); err != nil {
		s.log.Error("send reset password email failed", "user_id", user.ID, "error", err)
	}
	if s.cfg.Auth.ExposeDebugTokens && !s.email.Configured() {
		return raw, nil
	}
	return "", nil
}

func (s *AuthService) ResetPassword(ctx context.Context, rawToken, password string, meta SessionMeta) (*AuthResult, error) {
	if err := security.ValidatePassword(password); err != nil {
		return nil, domain.Invalid("weak_password", err.Error())
	}
	token, err := s.tokens.ConsumeOneTime(ctx, domain.TokenPurposeResetPassword, security.HashToken(rawToken), time.Now())
	if err != nil {
		return nil, domain.Invalid("invalid_reset_token", "Liên kết đặt lại mật khẩu không hợp lệ hoặc đã được sử dụng.")
	}
	user, err := s.users.FindByID(ctx, token.UserID)
	if err != nil {
		return nil, domain.Invalid("invalid_reset_token", "Không tìm thấy tài khoản cho liên kết này.")
	}
	hash, err := security.HashPassword(password, s.cfg.Auth.BcryptCost)
	if err != nil {
		return nil, err
	}
	if err := s.users.SetPasswordHash(ctx, user.ID, hash); err != nil {
		return nil, err
	}
	if !user.EmailVerified {
		_ = s.users.MarkEmailVerified(ctx, user.ID)
		user.EmailVerified = true
	}
	if err := s.sessions.RevokeAll(ctx, user.ID); err != nil {
		return nil, err
	}
	return s.sessions.Issue(ctx, user, meta)
}

func (s *AuthService) VerifyEmail(ctx context.Context, rawToken string) (*domain.User, error) {
	token, err := s.tokens.ConsumeOneTime(ctx, domain.TokenPurposeVerifyEmail, security.HashToken(rawToken), time.Now())
	if err != nil {
		return nil, domain.Invalid("invalid_verification_token", "Liên kết xác nhận không hợp lệ hoặc đã hết hạn.")
	}
	user, err := s.users.FindByID(ctx, token.UserID)
	if err != nil {
		return nil, domain.Invalid("invalid_verification_token", "Không tìm thấy tài khoản cho liên kết này.")
	}
	if err := s.users.MarkEmailVerified(ctx, user.ID); err != nil {
		return nil, err
	}
	user.EmailVerified = true
	go s.sendWelcome(user)
	return user.Public(), nil
}

func (s *AuthService) ResendVerification(ctx context.Context, email string) error {
	user, err := s.users.FindByEmail(ctx, normalizeEmail(email))
	if err != nil || user.EmailVerified {
		return nil
	}
	return s.sendVerification(ctx, user)
}

func (s *AuthService) sendVerification(ctx context.Context, user *domain.User) error {
	raw, err := security.RandomToken(32)
	if err != nil {
		return err
	}
	token := &domain.OneTimeToken{
		ID:        "ot_" + uuid.NewString(),
		UserID:    user.ID,
		Email:     user.Email,
		Purpose:   domain.TokenPurposeVerifyEmail,
		TokenHash: security.HashToken(raw),
		ExpiresAt: time.Now().Add(s.cfg.Auth.VerifyEmailTTL),
		CreatedAt: time.Now(),
	}
	if err := s.tokens.CreateOneTime(ctx, token); err != nil {
		return err
	}
	return s.email.SendVerifyEmail(ctx, user, raw)
}

func (s *AuthService) sendWelcome(user *domain.User) {
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	if err := s.email.SendWelcome(ctx, user); err != nil {
		s.log.Warn("send welcome email failed", "user_id", user.ID, "error", err)
	}
}

func normalizeEmail(value string) string {
	return strings.ToLower(strings.TrimSpace(value))
}
