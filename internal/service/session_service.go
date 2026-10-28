package service

import (
	"context"
	"time"

	"github.com/deepsi/agent-service/internal/config"
	"github.com/deepsi/agent-service/internal/domain"
	"github.com/deepsi/agent-service/internal/platform/security"
	"github.com/google/uuid"
)

type AuthResult struct {
	User                      *domain.User `json:"user,omitempty"`
	AccessToken               string       `json:"access_token,omitempty"`
	RefreshToken              string       `json:"refresh_token,omitempty"`
	TokenType                 string       `json:"token_type,omitempty"`
	ExpiresIn                 int64        `json:"expires_in,omitempty"`
	RequiresEmailVerification bool         `json:"requires_email_verification,omitempty"`
	Message                   string       `json:"message,omitempty"`
}

type SessionMeta struct {
	UserAgent string
	IP        string
}

type SessionService struct {
	cfg    *config.Config
	tokens domain.TokenRepository
	users  domain.UserRepository
	issuer *security.TokenIssuer
}

func NewSessionService(cfg *config.Config, tokens domain.TokenRepository, users domain.UserRepository) *SessionService {
	return &SessionService{
		cfg:    cfg,
		tokens: tokens,
		users:  users,
		issuer: security.NewTokenIssuer(cfg.Auth.JWTSecret, cfg.Auth.Issuer, cfg.Auth.AccessTTL),
	}
}

func (s *SessionService) Issue(ctx context.Context, user *domain.User, meta SessionMeta) (*AuthResult, error) {
	access, expiresAt, err := s.issuer.Issue(user.ID, user.Role)
	if err != nil {
		return nil, err
	}
	rawRefresh, err := security.RandomToken(32)
	if err != nil {
		return nil, err
	}
	refresh := &domain.RefreshToken{
		ID:        "rt_" + uuid.NewString(),
		UserID:    user.ID,
		TokenHash: security.HashToken(rawRefresh),
		ExpiresAt: time.Now().Add(s.cfg.Auth.RefreshTTL),
		UserAgent: truncate(meta.UserAgent, 300),
		IP:        meta.IP,
		CreatedAt: time.Now(),
	}
	if err := s.tokens.CreateRefresh(ctx, refresh); err != nil {
		return nil, err
	}
	return &AuthResult{
		User:         user.Public(),
		AccessToken:  access,
		RefreshToken: rawRefresh,
		TokenType:    "Bearer",
		ExpiresIn:    int64(time.Until(expiresAt).Seconds()),
	}, nil
}

func (s *SessionService) Rotate(ctx context.Context, rawRefresh string, meta SessionMeta) (*AuthResult, error) {
	hash := security.HashToken(rawRefresh)
	stored, err := s.tokens.FindRefreshByHash(ctx, hash)
	if err != nil {
		return nil, domain.Unauthorized("invalid_refresh_token", "Phiên đăng nhập không hợp lệ hoặc đã hết hạn.")
	}
	now := time.Now()
	if stored.RevokedAt != nil || stored.ExpiresAt.Before(now) {
		return nil, domain.Unauthorized("invalid_refresh_token", "Phiên đăng nhập không hợp lệ hoặc đã hết hạn.")
	}
	// Rotation: the old token can never be reused after this point.
	if err := s.tokens.RevokeRefresh(ctx, hash, now); err != nil {
		return nil, err
	}
	user, err := s.users.FindByID(ctx, stored.UserID)
	if err != nil {
		return nil, domain.Unauthorized("invalid_refresh_token", "Phiên đăng nhập không hợp lệ hoặc đã hết hạn.")
	}
	return s.Issue(ctx, user, meta)
}

func (s *SessionService) Revoke(ctx context.Context, rawRefresh string) error {
	if rawRefresh == "" {
		return nil
	}
	return s.tokens.RevokeRefresh(ctx, security.HashToken(rawRefresh), time.Now())
}

func (s *SessionService) RevokeAll(ctx context.Context, userID string) error {
	return s.tokens.RevokeAllRefreshForUser(ctx, userID, time.Now())
}

func (s *SessionService) ParseAccess(raw string) (*security.Claims, error) {
	return s.issuer.Parse(raw)
}

func (s *SessionService) IssueOAuthCode(ctx context.Context, user *domain.User, provider string) (string, error) {
	raw, err := security.RandomToken(32)
	if err != nil {
		return "", err
	}
	token := &domain.OneTimeToken{
		ID:        "ot_" + uuid.NewString(),
		UserID:    user.ID,
		Email:     user.Email,
		Purpose:   domain.TokenPurposeOAuthCode,
		TokenHash: security.HashToken(raw),
		Metadata:  map[string]string{"provider": provider},
		ExpiresAt: time.Now().Add(s.cfg.Auth.OAuthCodeTTL),
		CreatedAt: time.Now(),
	}
	if err := s.tokens.CreateOneTime(ctx, token); err != nil {
		return "", err
	}
	return raw, nil
}

func (s *SessionService) ConsumeOAuthCode(ctx context.Context, raw string) (*domain.OneTimeToken, error) {
	return s.tokens.ConsumeOneTime(ctx, domain.TokenPurposeOAuthCode, security.HashToken(raw), time.Now())
}

func truncate(value string, max int) string {
	if len(value) <= max {
		return value
	}
	return value[:max]
}
