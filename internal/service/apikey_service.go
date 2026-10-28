package service

import (
	"context"
	"strings"
	"time"

	"github.com/deepsi/agent-service/internal/config"
	"github.com/deepsi/agent-service/internal/domain"
	"github.com/deepsi/agent-service/internal/platform/security"
	"github.com/google/uuid"
)

type APIKeyService struct {
	cfg   *config.Config
	keys  domain.APIKeyRepository
	users domain.UserRepository
}

func NewAPIKeyService(cfg *config.Config, keys domain.APIKeyRepository, users domain.UserRepository) *APIKeyService {
	return &APIKeyService{cfg: cfg, keys: keys, users: users}
}

func (s *APIKeyService) List(ctx context.Context, userID string) ([]*domain.APIKey, error) {
	keys, err := s.keys.ListByUser(ctx, userID)
	if err != nil {
		return nil, err
	}
	for _, key := range keys {
		key.Plaintext = ""
	}
	return keys, nil
}

func (s *APIKeyService) Create(ctx context.Context, user *domain.User, input domain.CreateAPIKeyInput) (*domain.APIKey, error) {
	name := strings.TrimSpace(input.Name)
	if len([]rune(name)) < 2 || len([]rune(name)) > 80 {
		return nil, domain.Invalid("invalid_key_name", "Đặt cho key một cái tên dễ nhớ, từ 2 đến 80 ký tự.")
	}
	environment := strings.ToLower(strings.TrimSpace(input.Environment))
	if environment != domain.EnvProduction && environment != domain.EnvDevelopment {
		return nil, domain.Invalid("invalid_environment", "Môi trường key không hợp lệ.")
	}
	scopes := make([]string, 0, len(input.Scopes))
	seen := map[string]bool{}
	for _, scope := range input.Scopes {
		scope = strings.ToLower(strings.TrimSpace(scope))
		if scope == "" || seen[scope] {
			continue
		}
		if !domain.AllowedScopes[scope] {
			return nil, domain.Invalid("invalid_scope", "Quyền truy cập của key không hợp lệ.")
		}
		seen[scope] = true
		scopes = append(scopes, scope)
	}
	if len(scopes) == 0 {
		scopes = []string{"chat"}
	}

	plan := s.cfg.Plans.MustGet(user.EffectivePlan(time.Now()))
	active, err := s.keys.CountActive(ctx, user.ID)
	if err != nil {
		return nil, err
	}
	if !plan.UnlimitedKeys() && active >= int64(plan.KeyLimit) {
		return nil, domain.NewError(429, "key_limit_reached", "Gói "+plan.Name+" đã đạt giới hạn API key. Nâng cấp để tạo thêm.", nil)
	}

	raw, hash, prefix, last4, err := security.GenerateAPIKey(environment, s.cfg.Auth.APIKeyPepper)
	if err != nil {
		return nil, err
	}
	var expiresAt *time.Time
	if input.ExpiresDays > 0 {
		value := time.Now().AddDate(0, 0, input.ExpiresDays)
		expiresAt = &value
	}
	key := &domain.APIKey{
		ID:          "key_" + uuid.NewString(),
		UserID:      user.ID,
		Name:        name,
		Prefix:      prefix,
		Last4:       last4,
		KeyHash:     hash,
		Environment: environment,
		Scopes:      scopes,
		ExpiresAt:   expiresAt,
		Status:      domain.KeyStatusActive,
		CreatedAt:   time.Now(),
		Plaintext:   raw,
	}
	if err := s.keys.Create(ctx, key); err != nil {
		return nil, err
	}
	return key, nil
}

func (s *APIKeyService) Revoke(ctx context.Context, userID, keyID string) error {
	return s.keys.Revoke(ctx, userID, keyID, time.Now())
}

// Authenticate resolves a Bearer key for the OpenAI-compatible API. It returns
// the owner so quota checks can use their plan.
func (s *APIKeyService) Authenticate(ctx context.Context, raw string) (*domain.User, *domain.APIKey, error) {
	if !security.IsAPIKey(raw) {
		return nil, nil, domain.Unauthorized("invalid_api_key", "API key không hợp lệ.")
	}
	hash := security.HashAPIKey(raw, s.cfg.Auth.APIKeyPepper)
	key, err := s.keys.FindByHash(ctx, hash)
	if err != nil {
		return nil, nil, domain.Unauthorized("invalid_api_key", "API key không hợp lệ hoặc đã bị thu hồi.")
	}
	if key.Status != domain.KeyStatusActive {
		return nil, nil, domain.Unauthorized("invalid_api_key", "API key không hợp lệ hoặc đã bị thu hồi.")
	}
	if key.ExpiresAt != nil && key.ExpiresAt.Before(time.Now()) {
		return nil, nil, domain.Unauthorized("expired_api_key", "API key đã hết hạn.")
	}
	user, err := s.users.FindByID(ctx, key.UserID)
	if err != nil {
		return nil, nil, domain.Unauthorized("invalid_api_key", "API key không hợp lệ hoặc đã bị thu hồi.")
	}
	_ = s.keys.TouchLastUsed(ctx, key.ID, time.Now())
	return user, key, nil
}

func (s *APIKeyService) CountActive(ctx context.Context, userID string) (int64, error) {
	return s.keys.CountActive(ctx, userID)
}
