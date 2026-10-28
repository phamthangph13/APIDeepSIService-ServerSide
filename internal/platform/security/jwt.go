package security

import (
	"errors"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

type Claims struct {
	UserID   string `json:"uid"`
	Role     string `json:"role"`
	Type     string `json:"typ"`
	Provider string `json:"provider,omitempty"`
	Next     string `json:"next,omitempty"`
	Email    string `json:"email,omitempty"`
	jwt.RegisteredClaims
}

type TokenIssuer struct {
	secret []byte
	issuer string
	ttl    time.Duration
}

func NewTokenIssuer(secret, issuer string, ttl time.Duration) *TokenIssuer {
	return &TokenIssuer{secret: []byte(secret), issuer: issuer, ttl: ttl}
}

func (t *TokenIssuer) Issue(userID, role string) (string, time.Time, error) {
	now := time.Now()
	expires := now.Add(t.ttl)
	claims := Claims{
		UserID: userID,
		Role:   role,
		Type:   "access",
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    t.issuer,
			Subject:   userID,
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(expires),
			NotBefore: jwt.NewNumericDate(now.Add(-5 * time.Second)),
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, err := token.SignedString(t.secret)
	return signed, expires, err
}

func (t *TokenIssuer) Parse(raw string) (*Claims, error) {
	claims := &Claims{}
	token, err := jwt.ParseWithClaims(raw, claims, func(token *jwt.Token) (any, error) {
		if token.Method != jwt.SigningMethodHS256 {
			return nil, errors.New("unexpected signing method")
		}
		return t.secret, nil
	}, jwt.WithIssuer(t.issuer), jwt.WithExpirationRequired())
	if err != nil {
		return nil, err
	}
	if !token.Valid || claims.Type != "access" || claims.UserID == "" {
		return nil, errors.New("invalid token")
	}
	return claims, nil
}

// SignedOAuthState protects provider/next values across the OAuth redirect. It
// is not an access token and expires quickly.
func (t *TokenIssuer) SignedOAuthState(provider, next string, ttl time.Duration) (string, error) {
	now := time.Now()
	claims := Claims{
		Type:     "oauth_state",
		Provider: provider,
		Next:     next,
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    t.issuer,
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(ttl)),
		},
	}
	return jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(t.secret)
}

func (t *TokenIssuer) ParseState(raw, stateType string) (*Claims, error) {
	claims := &Claims{}
	token, err := jwt.ParseWithClaims(raw, claims, func(token *jwt.Token) (any, error) {
		if token.Method != jwt.SigningMethodHS256 {
			return nil, errors.New("unexpected signing method")
		}
		return t.secret, nil
	}, jwt.WithIssuer(t.issuer), jwt.WithExpirationRequired())
	if err != nil {
		return nil, err
	}
	if !token.Valid || claims.Type != stateType {
		return nil, errors.New("invalid state")
	}
	return claims, nil
}
