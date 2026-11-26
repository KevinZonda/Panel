package dao

import (
	"time"

	"github.com/KevinZonda/Panel/models/dbmod"
	"gorm.io/gorm"
)

func CreateSession(db *gorm.DB, session *dbmod.AuthSession) error {
	return db.Create(session).Error
}

func FindSessionByTokenHash(db *gorm.DB, tokenHash string) (*dbmod.AuthSession, error) {
	var session dbmod.AuthSession
	if err := db.Preload("User").
		Where("token_hash = ?", tokenHash).
		First(&session).Error; err != nil {
		return nil, err
	}
	return &session, nil
}

func UpdateSessionLastSeen(db *gorm.DB, sessionID uint, at time.Time) error {
	return db.Model(&dbmod.AuthSession{}).
		Where("id = ?", sessionID).
		Update("last_seen_at", at).Error
}

func RevokeSession(db *gorm.DB, sessionID uint, at time.Time) error {
	return db.Model(&dbmod.AuthSession{}).
		Where("id = ? AND revoked_at IS NULL", sessionID).
		Update("revoked_at", at).Error
}

func RevokeUserSessions(db *gorm.DB, userID uint, exceptSessionID uint, at time.Time) error {
	query := db.Model(&dbmod.AuthSession{}).
		Where("user_id = ? AND revoked_at IS NULL", userID)
	if exceptSessionID != 0 {
		query = query.Where("id <> ?", exceptSessionID)
	}
	return query.Update("revoked_at", at).Error
}
