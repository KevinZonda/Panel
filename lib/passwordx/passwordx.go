package passwordx

import (
	"fmt"
	"regexp"

	"golang.org/x/crypto/bcrypt"
)

var bcryptHashPattern = regexp.MustCompile(`^\$2[abxy]\$(0[4-9]|[12][0-9]|3[01])\$[./A-Za-z0-9]{53}$`)

const (
	MinPasswordBytes = 8
	MaxPasswordBytes = 72
)

func ValidatePassword(password string) error {
	if len(password) < MinPasswordBytes {
		return fmt.Errorf("password must be at least %d bytes", MinPasswordBytes)
	}
	if len(password) > MaxPasswordBytes {
		return fmt.Errorf("password must be at most %d bytes", MaxPasswordBytes)
	}
	return nil
}

func Hash(password string) (string, error) {
	if err := ValidatePassword(password); err != nil {
		return "", err
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return "", err
	}
	return string(hash), nil
}

func Compare(hash, password string) bool {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)) == nil
}

func ValidateHash(hash string) error {
	if !bcryptHashPattern.MatchString(hash) {
		return fmt.Errorf("invalid bcrypt password hash format")
	}
	cost, err := bcrypt.Cost([]byte(hash))
	if err != nil {
		return fmt.Errorf("invalid bcrypt password hash: %w", err)
	}
	if cost < bcrypt.DefaultCost {
		return fmt.Errorf("bcrypt password hash cost %d is below minimum %d", cost, bcrypt.DefaultCost)
	}
	return nil
}
