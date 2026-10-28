package security

import (
	"strings"
	"testing"
)

func TestValidatePassword(t *testing.T) {
	tests := []struct {
		name     string
		password string
		wantErr  bool
	}{
		{"short", "1234567", true},
		{"valid", "mat-khau-du-dai", false},
		{"too long", strings.Repeat("a", 73), true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := ValidatePassword(test.password)
			if (err != nil) != test.wantErr {
				t.Fatalf("ValidatePassword() error = %v, wantErr %v", err, test.wantErr)
			}
		})
	}
}

func TestGenerateAPIKey(t *testing.T) {
	raw, hash, prefix, last4, err := GenerateAPIKey("production", "pepper")
	if err != nil {
		t.Fatalf("GenerateAPIKey() error = %v", err)
	}
	if !IsAPIKey(raw) {
		t.Fatalf("generated key %q is not recognized", raw)
	}
	if !strings.HasPrefix(raw, "dsi_live_") {
		t.Fatalf("production key prefix = %q", raw)
	}
	if !strings.HasPrefix(prefix, "dsi_live_") {
		t.Fatalf("display prefix = %q", prefix)
	}
	if !strings.HasSuffix(raw, last4) {
		t.Fatalf("last4 %q does not match key", last4)
	}
	if hash != HashAPIKey(raw, "pepper") {
		t.Fatal("hash does not match HashAPIKey")
	}
	if hash == HashAPIKey(raw, "other-pepper") {
		t.Fatal("pepper must change the hash")
	}

	testRaw, _, _, _, err := GenerateAPIKey("development", "pepper")
	if err != nil {
		t.Fatalf("GenerateAPIKey() error = %v", err)
	}
	if !strings.HasPrefix(testRaw, "dsi_test_") {
		t.Fatalf("development key prefix = %q", testRaw)
	}
}

func TestHashTokenIsDeterministic(t *testing.T) {
	first := HashToken("token")
	second := HashToken("token")
	if first != second {
		t.Fatal("HashToken must be deterministic")
	}
	if first == HashToken("other") {
		t.Fatal("different tokens must have different hashes")
	}
}

func TestAvatarColor(t *testing.T) {
	for i := 0; i < 20; i++ {
		color := AvatarColor(i)
		if color == "" {
			t.Fatal("AvatarColor returned an empty color")
		}
	}
	if AvatarColor(0) != AvatarColor(5) {
		t.Fatal("AvatarColor should cycle")
	}
}
