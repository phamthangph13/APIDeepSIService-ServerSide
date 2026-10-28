package domain

import "time"

const (
	KeyStatusActive  = "active"
	KeyStatusRevoked = "revoked"

	EnvProduction  = "production"
	EnvDevelopment = "development"
)

var AllowedScopes = map[string]bool{
	"chat":       true,
	"embeddings": true,
	"vision":     true,
	"models":     true,
}

type APIKey struct {
	ID          string     `bson:"public_id" json:"id"`
	UserID      string     `bson:"user_id" json:"-"`
	Name        string     `bson:"name" json:"name"`
	Prefix      string     `bson:"prefix" json:"prefix"`
	Last4       string     `bson:"last4" json:"last4"`
	KeyHash     string     `bson:"key_hash" json:"-"`
	Environment string     `bson:"environment" json:"environment"`
	Scopes      []string   `bson:"scopes" json:"scopes"`
	ExpiresAt   *time.Time `bson:"expires_at,omitempty" json:"expires_at,omitempty"`
	LastUsedAt  *time.Time `bson:"last_used_at,omitempty" json:"last_used_at,omitempty"`
	Status      string     `bson:"status" json:"status"`
	CreatedAt   time.Time  `bson:"created_at" json:"created_at"`
	RevokedAt   *time.Time `bson:"revoked_at,omitempty" json:"revoked_at,omitempty"`

	// Plaintext is only populated once, immediately after creation.
	Plaintext string `bson:"-" json:"key,omitempty"`
}

func (k *APIKey) Masked() string {
	if k.Prefix == "" {
		return "••••••••"
	}
	return k.Prefix + "••••••••" + k.Last4
}

func (k *APIKey) HasScope(scope string) bool {
	for _, item := range k.Scopes {
		if item == scope {
			return true
		}
	}
	return false
}

type CreateAPIKeyInput struct {
	Name        string
	Environment string
	Scopes      []string
	ExpiresDays int
}
