package service

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/deepsi/agent-service/internal/config"
	"github.com/deepsi/agent-service/internal/domain"
	"github.com/deepsi/agent-service/internal/platform/security"
	"github.com/google/uuid"
	"golang.org/x/oauth2"
)

type OAuthService struct {
	cfg      *config.Config
	users    domain.UserRepository
	sessions *SessionService
	issuer   *security.TokenIssuer
	client   *http.Client
	log      *slog.Logger
}

type oauthProvider struct {
	name        string
	config      config.OAuthProviderConfig
	oauth       *oauth2.Config
	userInfoURL string
	emailURL    string
	scopes      []string
	authStyle   oauth2.AuthStyle
}

func NewOAuthService(cfg *config.Config, users domain.UserRepository, sessions *SessionService, log *slog.Logger) *OAuthService {
	return &OAuthService{
		cfg:      cfg,
		users:    users,
		sessions: sessions,
		issuer:   security.NewTokenIssuer(cfg.Auth.JWTSecret, cfg.Auth.Issuer, cfg.Auth.AccessTTL),
		client:   &http.Client{Timeout: 20 * time.Second},
		log:      log,
	}
}

func (s *OAuthService) StartURL(provider, next string) (string, error) {
	p, err := s.provider(provider)
	if err != nil {
		return "", err
	}
	if p.config.ClientID == "" || p.config.ClientSecret == "" {
		return "", domain.NewError(http.StatusServiceUnavailable, "oauth_not_configured", fmt.Sprintf("Đăng nhập %s chưa được cấu hình.", provider), nil)
	}
	next = sanitizeNext(next)
	state, err := s.issuer.SignedOAuthState(provider, next, 10*time.Minute)
	if err != nil {
		return "", err
	}
	opts := []oauth2.AuthCodeOption{oauth2.AccessTypeOnline}
	if provider == domain.ProviderGoogle {
		opts = append(opts, oauth2.SetAuthURLParam("prompt", "select_account"))
	}
	return p.oauth.AuthCodeURL(state, opts...), nil
}

func (s *OAuthService) HandleCallback(ctx context.Context, provider, code, state string) (string, error) {
	p, err := s.provider(provider)
	if err != nil {
		return "", err
	}
	claims, err := s.issuer.ParseState(state, "oauth_state")
	if err != nil || claims.Provider != provider {
		return "", domain.Invalid("invalid_oauth_state", "Phiên đăng nhập OAuth không hợp lệ hoặc đã hết hạn.")
	}
	if code == "" {
		return "", domain.Invalid("missing_oauth_code", "Nhà cung cấp không trả về mã đăng nhập.")
	}
	token, err := p.oauth.Exchange(ctx, code)
	if err != nil {
		return "", domain.NewError(http.StatusBadGateway, "oauth_exchange_failed", "Không thể xác thực với nhà cung cấp OAuth.", err)
	}
	profile, err := s.fetchProfile(ctx, p, token)
	if err != nil {
		return "", err
	}
	if profile.Email == "" {
		return "", domain.NewError(http.StatusBadGateway, "oauth_email_missing", "Không lấy được email từ nhà cung cấp OAuth.", nil)
	}
	user, err := s.upsertUser(ctx, provider, profile)
	if err != nil {
		return "", err
	}
	oneTimeCode, err := s.sessions.IssueOAuthCode(ctx, user, provider)
	if err != nil {
		return "", err
	}
	redirect := fmt.Sprintf("%s/auth/callback?code=%s&next=%s",
		s.cfg.App.FrontendURL,
		url.QueryEscape(oneTimeCode),
		url.QueryEscape(sanitizeNext(claims.Next)),
	)
	return redirect, nil
}

func (s *OAuthService) ExchangeCode(ctx context.Context, code string, meta SessionMeta) (*AuthResult, error) {
	if code == "" {
		return nil, domain.Invalid("missing_oauth_code", "Thiếu mã đăng nhập OAuth.")
	}
	token, err := s.sessions.ConsumeOAuthCode(ctx, code)
	if err != nil {
		return nil, domain.Invalid("invalid_oauth_code", "Mã đăng nhập OAuth không hợp lệ hoặc đã hết hạn.")
	}
	user, err := s.users.FindByID(ctx, token.UserID)
	if err != nil {
		return nil, domain.Unauthorized("user_not_found", "Tài khoản không tồn tại.")
	}
	_ = s.users.TouchLogin(ctx, user.ID, time.Now())
	return s.sessions.Issue(ctx, user, meta)
}

type oauthProfile struct {
	ID            string
	Email         string
	EmailVerified bool
	Name          string
}

func (s *OAuthService) provider(name string) (*oauthProvider, error) {
	switch strings.ToLower(name) {
	case domain.ProviderGoogle:
		cfg := s.cfg.OAuth.Google
		redirect := cfg.RedirectURL
		if redirect == "" {
			redirect = s.cfg.App.PublicAPIURL + "/api/v1/auth/oauth/google/callback"
		}
		return &oauthProvider{
			name:   domain.ProviderGoogle,
			config: cfg,
			oauth: &oauth2.Config{
				ClientID:     cfg.ClientID,
				ClientSecret: cfg.ClientSecret,
				RedirectURL:  redirect,
				Scopes:       cfg.Scopes,
				Endpoint:     oauth2.Endpoint{AuthURL: cfg.AuthURL, TokenURL: cfg.TokenURL, AuthStyle: oauth2.AuthStyleInParams},
			},
			userInfoURL: cfg.UserInfoURL,
			scopes:      cfg.Scopes,
		}, nil
	case domain.ProviderGitHub:
		cfg := s.cfg.OAuth.GitHub
		redirect := cfg.RedirectURL
		if redirect == "" {
			redirect = s.cfg.App.PublicAPIURL + "/api/v1/auth/oauth/github/callback"
		}
		return &oauthProvider{
			name:   domain.ProviderGitHub,
			config: cfg,
			oauth: &oauth2.Config{
				ClientID:     cfg.ClientID,
				ClientSecret: cfg.ClientSecret,
				RedirectURL:  redirect,
				Scopes:       cfg.Scopes,
				Endpoint:     oauth2.Endpoint{AuthURL: cfg.AuthURL, TokenURL: cfg.TokenURL, AuthStyle: oauth2.AuthStyleInParams},
			},
			userInfoURL: cfg.UserInfoURL,
			emailURL:    cfg.EmailURL,
			scopes:      cfg.Scopes,
		}, nil
	default:
		return nil, domain.NotFound("oauth_provider_not_found", "Nhà cung cấp đăng nhập không hợp lệ.")
	}
}

func (s *OAuthService) fetchProfile(ctx context.Context, p *oauthProvider, token *oauth2.Token) (*oauthProfile, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, p.userInfoURL, nil)
	if err != nil {
		return nil, err
	}
	request.Header.Set("Authorization", "Bearer "+token.AccessToken)
	request.Header.Set("Accept", "application/json")
	request.Header.Set("User-Agent", "DeepSI-Agent-Service/1.0")
	response, err := s.client.Do(request)
	if err != nil {
		return nil, domain.NewError(http.StatusBadGateway, "oauth_profile_failed", "Không lấy được thông tin tài khoản từ nhà cung cấp OAuth.", err)
	}
	defer response.Body.Close()
	body, err := io.ReadAll(io.LimitReader(response.Body, 1<<20))
	if err != nil {
		return nil, err
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return nil, domain.NewError(http.StatusBadGateway, "oauth_profile_failed", "Nhà cung cấp OAuth trả về lỗi.", fmt.Errorf("status %d: %s", response.StatusCode, string(body)))
	}

	switch p.name {
	case domain.ProviderGoogle:
		var payload struct {
			Sub           string `json:"sub"`
			Email         string `json:"email"`
			EmailVerified bool   `json:"email_verified"`
			Name          string `json:"name"`
		}
		if err := json.Unmarshal(body, &payload); err != nil {
			return nil, err
		}
		return &oauthProfile{ID: payload.Sub, Email: normalizeEmail(payload.Email), EmailVerified: payload.EmailVerified, Name: payload.Name}, nil
	case domain.ProviderGitHub:
		var payload struct {
			ID    int64  `json:"id"`
			Login string `json:"login"`
			Name  string `json:"name"`
			Email string `json:"email"`
		}
		if err := json.Unmarshal(body, &payload); err != nil {
			return nil, err
		}
		email := normalizeEmail(payload.Email)
		verified := false
		if email == "" && p.emailURL != "" {
			email, verified, err = s.fetchGitHubEmail(ctx, p, token)
			if err != nil {
				s.log.Warn("fetch github email failed", "error", err)
			}
		}
		name := payload.Name
		if name == "" {
			name = payload.Login
		}
		return &oauthProfile{ID: fmt.Sprintf("%d", payload.ID), Email: email, EmailVerified: verified, Name: name}, nil
	default:
		return nil, domain.Internal(fmt.Errorf("unsupported oauth provider %s", p.name))
	}
}

func (s *OAuthService) fetchGitHubEmail(ctx context.Context, p *oauthProvider, token *oauth2.Token) (string, bool, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, p.emailURL, nil)
	if err != nil {
		return "", false, err
	}
	request.Header.Set("Authorization", "Bearer "+token.AccessToken)
	request.Header.Set("Accept", "application/vnd.github+json")
	request.Header.Set("User-Agent", "DeepSI-Agent-Service/1.0")
	response, err := s.client.Do(request)
	if err != nil {
		return "", false, err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return "", false, fmt.Errorf("github emails status %d", response.StatusCode)
	}
	var emails []struct {
		Email    string `json:"email"`
		Primary  bool   `json:"primary"`
		Verified bool   `json:"verified"`
	}
	if err := json.NewDecoder(io.LimitReader(response.Body, 1<<20)).Decode(&emails); err != nil {
		return "", false, err
	}
	var fallback string
	for _, item := range emails {
		if item.Verified && item.Primary {
			return normalizeEmail(item.Email), true, nil
		}
		if item.Verified && fallback == "" {
			fallback = normalizeEmail(item.Email)
		}
	}
	if fallback != "" {
		return fallback, true, nil
	}
	return "", false, nil
}

func (s *OAuthService) upsertUser(ctx context.Context, provider string, profile *oauthProfile) (*domain.User, error) {
	if profile.ID == "" {
		return nil, domain.NewError(http.StatusBadGateway, "oauth_profile_invalid", "Nhà cung cấp OAuth không trả về mã tài khoản.", nil)
	}
	if user, err := s.users.FindByProvider(ctx, provider, profile.ID); err == nil {
		return user, nil
	}
	if user, err := s.users.FindByEmail(ctx, profile.Email); err == nil {
		user.Provider = provider
		user.ProviderID = profile.ID
		user.EmailVerified = true
		if user.Name == "" {
			user.Name = profile.Name
		}
		if err := s.users.Update(ctx, user); err != nil {
			return nil, err
		}
		return user, nil
	}

	name := strings.TrimSpace(profile.Name)
	if name == "" {
		name = strings.Split(profile.Email, "@")[0]
	}
	count, _ := s.users.Count(ctx)
	user := &domain.User{
		ID:            "usr_" + uuid.NewString(),
		Name:          name,
		Email:         profile.Email,
		Provider:      provider,
		ProviderID:    profile.ID,
		Role:          domain.RoleClient,
		Plan:          domain.PlanFree,
		EmailVerified: true,
		AvatarColor:   security.AvatarColor(int(count)),
	}
	if err := s.users.Create(ctx, user); err != nil {
		if err == domain.ErrDuplicate {
			// A concurrent callback may have created the same account.
			if existing, findErr := s.users.FindByEmail(ctx, profile.Email); findErr == nil {
				return existing, nil
			}
		}
		return nil, err
	}
	return user, nil
}

func sanitizeNext(next string) string {
	next = strings.TrimSpace(next)
	if next == "" || !strings.HasPrefix(next, "/") || strings.HasPrefix(next, "//") {
		return "/dashboard"
	}
	return next
}
