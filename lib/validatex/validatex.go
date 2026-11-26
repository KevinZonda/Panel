package validatex

import (
	"regexp"

	"github.com/KevinZonda/Panel/models"
)

var usernamePattern = regexp.MustCompile(`^[A-Za-z0-9._-]{1,64}$`)

func ValidUsername(username string) bool {
	return usernamePattern.MatchString(username)
}

func ValidRole(role string) bool {
	return role == models.RoleAdmin || role == models.RoleViewer
}

func ValidUserStatus(status string) bool {
	return status == models.UserStatusActive || status == models.UserStatusDisabled
}
