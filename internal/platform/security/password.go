package security

import (
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"

	"golang.org/x/crypto/bcrypt"
)

func HashPassword(password string, cost int) (string, error) {
	if cost < 10 {
		cost = 12
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(password), cost)
	if err != nil {
		return "", fmt.Errorf("hash password: %w", err)
	}
	return string(hash), nil
}

func CheckPassword(hash, password string) bool {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)) == nil
}

func RandomToken(bytes int) (string, error) {
	buf := make([]byte, bytes)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(buf), nil
}

func HashToken(raw string) string {
	sum := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(sum[:])
}

func ConstantTimeEqual(a, b string) bool {
	return subtle.ConstantTimeCompare([]byte(a), []byte(b)) == 1
}

// GenerateAPIKey returns a DeepSI key and its HMAC-SHA256 hash. The pepper is
// stored only in the environment, so a database leak alone cannot verify keys.
func GenerateAPIKey(environment, pepper string) (raw, hash, prefix, last4 string, err error) {
	envPrefix := "dsi_live"
	if environment == "development" {
		envPrefix = "dsi_test"
	}
	secret, err := RandomToken(27) // 36 base64url characters
	if err != nil {
		return "", "", "", "", err
	}
	raw = envPrefix + "_" + secret
	hash = HashAPIKey(raw, pepper)
	prefix = raw[:len(envPrefix)+1+4]
	last4 = raw[len(raw)-4:]
	return raw, hash, prefix, last4, nil
}

func HashAPIKey(raw, pepper string) string {
	sum := sha256.Sum256([]byte(pepper + ":" + raw))
	return hex.EncodeToString(sum[:])
}

func IsAPIKey(raw string) bool {
	return strings.HasPrefix(raw, "dsi_live_") || strings.HasPrefix(raw, "dsi_test_")
}

func ValidatePassword(password string) error {
	if len(password) < 8 {
		return errors.New("mật khẩu cần ít nhất 8 ký tự")
	}
	if len(password) > 72 {
		return errors.New("mật khẩu không được dài quá 72 ký tự")
	}
	return nil
}

func RandomHex(n int) (string, error) {
	buf := make([]byte, n)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return hex.EncodeToString(buf), nil
}

func AvatarColor(seed int) string {
	colors := []string{"moss", "clay", "ochre", "blue", "lavender"}
	if seed < 0 {
		seed = -seed
	}
	return colors[seed%len(colors)]
}
