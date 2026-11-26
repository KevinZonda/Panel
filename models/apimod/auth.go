package apimod

import (
	"time"

	"github.com/KevinZonda/Panel/models/dbmod"
)

type LoginRequest struct {
	Username string `json:"username" binding:"required,min=1,max=64"`
	Password string `json:"password" binding:"required,min=1,max=256"`
}

type LoginResponse struct {
	Token     string    `json:"token"`
	TokenType string    `json:"token_type"`
	ExpiresAt time.Time `json:"expires_at"`
	User      User      `json:"user"`
}

type ChangePasswordRequest struct {
	OldPassword string `json:"old_password" binding:"required"`
	NewPassword string `json:"new_password" binding:"required,min=8,max=256"`
}

func UserResponse(user *dbmod.User) User {
	if user == nil {
		return User{}
	}
	return User{
		ID:          user.ID,
		Username:    user.Username,
		Role:        user.Role,
		Status:      user.Status,
		CreatedAt:   user.CreatedAt,
		UpdatedAt:   user.UpdatedAt,
		LastLoginAt: user.LastLoginAt,
	}
}
