package apimod

import "time"

type User struct {
	ID          uint       `json:"id"`
	Username    string     `json:"username"`
	Role        string     `json:"role"`
	Status      string     `json:"status"`
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
	LastLoginAt *time.Time `json:"last_login_at"`
}

type UserList struct {
	Items    []User `json:"items"`
	Page     int    `json:"page"`
	PageSize int    `json:"page_size"`
	Total    int64  `json:"total"`
}

type CreateUserRequest struct {
	Username string `json:"username" binding:"required,min=1,max=64"`
	Password string `json:"password" binding:"required,min=8,max=256"`
	Role     string `json:"role" binding:"required,oneof=admin viewer"`
	Status   string `json:"status" binding:"required,oneof=active disabled"`
}

type UpdateUserRequest struct {
	Username *string `json:"username" binding:"omitempty,min=1,max=64"`
	Role     *string `json:"role" binding:"omitempty,oneof=admin viewer"`
	Status   *string `json:"status" binding:"omitempty,oneof=active disabled"`
}

type ResetPasswordRequest struct {
	NewPassword string `json:"new_password" binding:"required,min=8,max=256"`
}
