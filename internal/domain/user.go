package domain

import "time"

const (
	RoleClient = "client"
	RoleAdmin  = "admin"

	ProviderPassword = "password"
	ProviderGoogle   = "google"
	ProviderGitHub   = "github"
)

type User struct {
	ID                string     `bson:"public_id" json:"id"`
	Name              string     `bson:"name" json:"name"`
	Email             string     `bson:"email" json:"email"`
	PasswordHash      string     `bson:"password_hash,omitempty" json:"-"`
	Provider          string     `bson:"provider" json:"provider"`
	ProviderID        string     `bson:"provider_id,omitempty" json:"-"`
	Role              string     `bson:"role" json:"role"`
	Plan              PlanID     `bson:"plan" json:"plan"`
	PlanStartedAt     *time.Time `bson:"plan_started_at,omitempty" json:"plan_started_at,omitempty"`
	PlanExpiresAt     *time.Time `bson:"plan_expires_at,omitempty" json:"plan_expires_at,omitempty"`
	CancelAtPeriodEnd bool       `bson:"cancel_at_period_end" json:"cancel_at_period_end"`
	EmailVerified     bool       `bson:"email_verified" json:"email_verified"`
	AvatarColor       string     `bson:"avatar_color" json:"avatar_color"`
	CreatedAt         time.Time  `bson:"created_at" json:"created_at"`
	UpdatedAt         time.Time  `bson:"updated_at" json:"updated_at"`
	LastLoginAt       *time.Time `bson:"last_login_at,omitempty" json:"last_login_at,omitempty"`
}

func (u *User) IsAdmin() bool { return u.Role == RoleAdmin }

func (u *User) IsPaid() bool {
	return u.Plan == PlanPlus || u.Plan == PlanUltral
}

// EffectivePlan returns free when a paid subscription has expired. It does not
// mutate the user; persistence is handled by the billing/auth services.
func (u *User) EffectivePlan(now time.Time) PlanID {
	if u.Plan == PlanPlus || u.Plan == PlanUltral {
		if u.PlanExpiresAt != nil && u.PlanExpiresAt.Before(now) {
			return PlanFree
		}
	}
	return u.Plan
}

func (u *User) Public() *User {
	copy := *u
	copy.PasswordHash = ""
	copy.ProviderID = ""
	return &copy
}

type UserFilter struct {
	Query  string
	Role   string
	Plan   string
	Limit  int64
	Offset int64
}

type AdminUserListItem struct {
	User      *User     `json:"user"`
	KeyCount  int64     `json:"key_count"`
	Tokens30d int64     `json:"tokens_30d"`
	LastSeen  time.Time `json:"last_seen"`
}
