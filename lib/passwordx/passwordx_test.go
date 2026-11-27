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
