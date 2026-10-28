package domain

import "time"

const (
	TokenPurposeResetPassword = "reset_password"
	TokenPurposeVerifyEmail   = "verify_email"
	TokenPurposeOAuthCode     = "oauth_code"
)

type RefreshToken struct {
	ID        string     `bson:"id"`
	UserID    string     `bson:"user_id"`
	TokenHash string     `bson:"token_hash"`
	ExpiresAt time.Time  `bson:"expires_at"`
	RevokedAt *time.Time `bson:"revoked_at,omitempty"`
	UserAgent string     `bson:"user_agent,omitempty"`
	IP        string     `bson:"ip,omitempty"`
	CreatedAt time.Time  `bson:"created_at"`
}

type OneTimeToken struct {
	ID        string            `bson:"id"`
	UserID    string            `bson:"user_id,omitempty"`
	Email     string            `bson:"email,omitempty"`
	Purpose   string            `bson:"purpose"`
	TokenHash string            `bson:"token_hash"`
	Metadata  map[string]string `bson:"metadata,omitempty"`
	ExpiresAt time.Time         `bson:"expires_at"`
	UsedAt    *time.Time        `bson:"used_at,omitempty"`
	CreatedAt time.Time         `bson:"created_at"`
}
