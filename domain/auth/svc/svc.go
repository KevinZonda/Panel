package svc

import (
	"errors"
	"fmt"
	"time"

	"github.com/KevinZonda/GoX/pkg/cryptox"
	authdao "github.com/KevinZonda/Panel/domain/auth/dao"
	userdao "github.com/KevinZonda/Panel/domain/user/dao"
	"github.com/KevinZonda/Panel/lib/apperr"
	"github.com/KevinZonda/Panel/lib/passwordx"
	"github.com/KevinZonda/Panel/lib/tokenx"
	"github.com/KevinZonda/Panel/lib/validatex"
	"github.com/KevinZonda/Panel/models"
	"github.com/KevinZonda/Panel/models/apimod"
	"github.com/KevinZonda/Panel/models/dbmod"
	"gorm.io/gorm"
)

type Service struct {
	db       *gorm.DB
	tokenTTL time.Duration
}

func NewService(db *gorm.DB, tokenTTL time.Duration) *Service {
	return &Service{db: db, tokenTTL: tokenTTL}
}

func (s *Service) EnsureBootstrapAdmin(username, passwordHash string) error {
	count, err := userdao.CountUsers(s.db)
	if err != nil {
		return err
	}
	if count > 0 {
		return nil
	}
	if passwordHash == "" {
		return fmt.Errorf("first admin requires a password hash; run panel init or set PANEL_BOOTSTRAP_ADMIN_PASSWORD_HASH")
	}
	if !validatex.ValidUsername(username) {
		return apperr.Validationf("bootstrap admin username is invalid")
	}
	if err := passwordx.ValidateHash(passwordHash); err != nil {
		return fmt.Errorf("invalid bootstrap_admin_password_hash: %w", err)
	}
	user := &dbmod.User{
		Username:     username,
		PasswordHash: passwordHash,
		Role:         models.RoleAdmin,
		Status:       models.UserStatusActive,
	}
	return userdao.CreateUser(s.db, user)
}

func (s *Service) Login(username, password string) (*apimod.LoginResponse, error) {
	user, err := userdao.FindUserByUsername(s.db, username)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, apperr.ErrInvalidCredentials
		}
		return nil, err
	}
	if !passwordx.Compare(user.PasswordHash, password) {
		return nil, apperr.ErrInvalidCredentials
	}
	if user.Status != models.UserStatusActive {
		return nil, apperr.ErrUserDisabled
	}

	token, err := tokenx.New(32)
	if err != nil {
		return nil, err
	}
	tokenHash := cryptox.SHA256Str(token)

	var expiresAt time.Time
	err = s.db.Transaction(func(tx *gorm.DB) error {
		currentUser, err := userdao.FindUserByID(tx, user.ID)
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return apperr.ErrInvalidCredentials
		}
		if err != nil {
			return err
		}
		if currentUser.PasswordHash != user.PasswordHash || currentUser.Username != user.Username {
			return apperr.ErrInvalidCredentials
		}
		if currentUser.Status != models.UserStatusActive {
			return apperr.ErrUserDisabled
		}

		now := time.Now()
		expiresAt = now.Add(s.tokenTTL)
		session := &dbmod.AuthSession{
			UserID:    currentUser.ID,
			TokenHash: tokenHash,
			CreatedAt: now,
			ExpiresAt: expiresAt,
		}
		if err := authdao.CreateSession(tx, session); err != nil {
			return err
		}
		if err := userdao.UpdateLastLoginAt(tx, currentUser.ID, now); err != nil {
			return err
		}
		currentUser.LastLoginAt = &now
		user = currentUser
		return nil
	})
	if err != nil {
		return nil, err
	}

	return &apimod.LoginResponse{
		Token:     token,
		TokenType: "Bearer",
		ExpiresAt: expiresAt,
		User:      apimod.UserResponse(user),
	}, nil
}

func (s *Service) ValidateToken(token string) (*dbmod.User, *dbmod.AuthSession, error) {
	if token == "" {
		return nil, nil, apperr.ErrInvalidCredentials
	}
	session, err := authdao.FindSessionByTokenHash(s.db, cryptox.SHA256Str(token))
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil, apperr.ErrInvalidCredentials
		}
		return nil, nil, err
	}

	now := time.Now()
	if session.RevokedAt != nil || !session.ExpiresAt.After(now) {
		return nil, nil, apperr.ErrInvalidCredentials
	}
	if session.User.Status != models.UserStatusActive {
		return nil, nil, apperr.ErrUserDisabled
	}

	if session.LastSeenAt == nil || session.LastSeenAt.Before(now.Add(-time.Minute)) {
		_ = authdao.UpdateSessionLastSeen(s.db, session.ID, now)
	}

	user := session.User
	return &user, session, nil
}

func (s *Service) Logout(sessionID uint) error {
	return authdao.RevokeSession(s.db, sessionID, time.Now())
}

func (s *Service) ChangePassword(userID uint, currentSessionID uint, oldPassword, newPassword string) error {
	return s.db.Transaction(func(tx *gorm.DB) error {
		user, err := userdao.FindUserByID(tx, userID)
		if err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return apperr.NotFoundf("user not found")
			}
			return err
		}
		if !passwordx.Compare(user.PasswordHash, oldPassword) {
			return apperr.ErrInvalidCredentials
		}

		if err := passwordx.ValidatePassword(newPassword); err != nil {
			return apperr.Validationf("%s", err)
		}
		hash, err := passwordx.Hash(newPassword)
		if err != nil {
			return err
		}
		user.PasswordHash = hash
		if err := userdao.UpdateUser(tx, user); err != nil {
			return err
		}
		return authdao.RevokeUserSessions(tx, userID, currentSessionID, time.Now())
	})
}

func (s *Service) RevokeUserSessions(userID uint) error {
	return authdao.RevokeUserSessions(s.db, userID, 0, time.Now())
}
