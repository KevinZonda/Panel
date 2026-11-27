package passwordx

import (
	"strings"
	"testing"

	"golang.org/x/crypto/bcrypt"
)

func TestValidateHash(t *testing.T) {
	hash, err := Hash("password123")
	if err != nil {
		t.Fatalf("hash password: %v", err)
	}
	if err := ValidateHash(hash); err != nil {
		t.Fatalf("expected valid bcrypt hash, got %v", err)
	}

	if err := ValidateHash("not-a-bcrypt-hash"); err == nil {
		t.Fatal("expected malformed hash to be rejected")
	}
	corruptedHash := hash[:len(hash)-1] + "!"
	if err := ValidateHash(corruptedHash); err == nil {
		t.Fatal("expected corrupted hash checksum to be rejected")
	}
	if err := ValidateHash(hash[:len(hash)-1]); err == nil {
		t.Fatal("expected truncated hash to be rejected")
	}

	lowCostHash, err := bcrypt.GenerateFromPassword([]byte("password123"), bcrypt.MinCost)
	if err != nil {
		t.Fatalf("hash password with low cost: %v", err)
	}
	err = ValidateHash(string(lowCostHash))
	if err == nil || !strings.Contains(err.Error(), "below minimum") {
		t.Fatalf("expected low cost hash to be rejected, got %v", err)
	}
}

func TestValidatePasswordUsesByteLength(t *testing.T) {
	for _, tc := range []struct {
		name     string
		password string
		valid    bool
	}{
		{"too short", "1234567", false},
		{"minimum", "12345678", true},
		{"maximum", strings.Repeat("a", 72), true},
		{"too long", strings.Repeat("a", 73), false},
		{"multibyte maximum", strings.Repeat("中", 24), true},
		{"multibyte too long", strings.Repeat("中", 25), false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			err := ValidatePassword(tc.password)
			if tc.valid && err != nil {
				t.Fatalf("expected valid password, got %v", err)
			}
			if !tc.valid && err == nil {
				t.Fatal("expected invalid password")
			}
		})
	}

	if _, err := Hash(strings.Repeat("a", 73)); err == nil {
		t.Fatal("expected overlong password to be rejected before bcrypt")
	}
}
