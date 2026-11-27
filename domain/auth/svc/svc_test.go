package svc_test

import (
	"errors"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/KevinZonda/Panel/domain/auth/svc"
	"github.com/KevinZonda/Panel/lib/apperr"
	"github.com/KevinZonda/Panel/lib/passwordx"
	"github.com/KevinZonda/Panel/models/dbmod"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestLoginValidateTokenAndLogout(t *testing.T) {
	db := newTestDB(t)
	service := svc.NewService(db, time.Hour)

	bootstrapHash, err := passwordx.Hash("password123")
	if err != nil {
		t.Fatalf("hash bootstrap password: %v", err)
	}
	if err := service.EnsureBootstrapAdmin("admin", bootstrapHash); err != nil {
		t.Fatalf("bootstrap admin: %v", err)
	}

	if _, err := service.Login("admin", "wrong-password"); !errors.Is(err, apperr.ErrInvalidCredentials) {
		t.Fatalf("expected invalid credentials, got %v", err)
	}
	var beforeLogin dbmod.User
	if err := db.Where("username = ?", "admin").First(&beforeLogin).Error; err != nil {
		t.Fatalf("load user before login: %v", err)
	}
	if beforeLogin.LastLoginAt != nil {
		t.Fatal("failed login must leave last_login_at unset")
	}
	if beforeLogin.PasswordHash != bootstrapHash {
		t.Fatal("bootstrap admin must store the configured bcrypt hash unchanged")
	}

	login, err := service.Login("admin", "password123")
	if err != nil {
		t.Fatalf("login: %v", err)
	}
	if login.Token == "" {
		t.Fatal("expected token")
	}

	user, session, err := service.ValidateToken(login.Token)
	if err != nil {
		t.Fatalf("validate token: %v", err)
	}
	if user.Username != "admin" || session.UserID != user.ID {
		t.Fatalf("unexpected user/session: %#v %#v", user, session)
	}
	if login.User.LastLoginAt == nil || user.LastLoginAt == nil || !user.LastLoginAt.Equal(*login.User.LastLoginAt) {
		t.Fatal("login response and stored user must have the same last_login_at")
	}
	if !user.UpdatedAt.Equal(beforeLogin.UpdatedAt) || !login.User.UpdatedAt.Equal(beforeLogin.UpdatedAt) {
		t.Fatal("login must leave updated_at unchanged")
	}

	if err := service.Logout(session.ID); err != nil {
		t.Fatalf("logout: %v", err)
	}
	if _, _, err := service.ValidateToken(login.Token); !errors.Is(err, apperr.ErrInvalidCredentials) {
		t.Fatalf("expected revoked token to be invalid, got %v", err)
	}
}

func TestEnsureBootstrapAdminRejectsInvalidHash(t *testing.T) {
	service := svc.NewService(newTestDB(t), time.Hour)
	if err := service.EnsureBootstrapAdmin("admin", "not-a-bcrypt-hash"); err == nil {
		t.Fatal("expected invalid bootstrap hash error")
	}
}

func TestChangePasswordRejectsOverlongPassword(t *testing.T) {
	service := svc.NewService(newTestDB(t), time.Hour)
	bootstrapHash, err := passwordx.Hash("password123")
	if err != nil {
		t.Fatalf("hash bootstrap password: %v", err)
	}
	if err := service.EnsureBootstrapAdmin("admin", bootstrapHash); err != nil {
		t.Fatalf("bootstrap admin: %v", err)
	}
	login, err := service.Login("admin", "password123")
	if err != nil {
		t.Fatalf("login: %v", err)
	}
	if err := service.ChangePassword(login.User.ID, 0, "password123", strings.Repeat("a", 73)); !errors.Is(err, apperr.ErrValidation) {
		t.Fatalf("expected validation error, got %v", err)
	}
}

func newTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	dsn := "file:" + filepath.Join(t.TempDir(), "panel-test.db") + "?_foreign_keys=on"
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("open test db: %v", err)
	}
	if err := db.AutoMigrate(&dbmod.User{}, &dbmod.AuthSession{}); err != nil {
		t.Fatalf("migrate test db: %v", err)
	}
	return db
}
