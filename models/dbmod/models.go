package dbmod

import "time"

type User struct {
	ID           uint      `gorm:"primaryKey"`
	Username     string    `gorm:"size:64;not null;uniqueIndex"`
	PasswordHash string    `gorm:"size:255;not null"`
	Role         string    `gorm:"size:16;not null;index"`
	Status       string    `gorm:"size:16;not null;index"`
	CreatedAt    time.Time `gorm:"not null"`
	UpdatedAt    time.Time `gorm:"not null"`
	LastLoginAt  *time.Time
}

type AuthSession struct {
	ID         uint      `gorm:"primaryKey"`
	UserID     uint      `gorm:"not null;index"`
	User       User      `gorm:"constraint:OnUpdate:CASCADE,OnDelete:CASCADE"`
	TokenHash  string    `gorm:"size:64;not null;uniqueIndex"`
	CreatedAt  time.Time `gorm:"not null"`
	ExpiresAt  time.Time `gorm:"not null;index"`
	LastSeenAt *time.Time
	RevokedAt  *time.Time `gorm:"index"`
}
