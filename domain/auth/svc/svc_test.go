package svc_test

import (
	"errors"
	"path/filepath"
	"testing"
	"time"

	"github.com/KevinZonda/Panel/domain/auth/svc"
	"github.com/KevinZonda/Panel/lib/apperr"
	"github.com/KevinZonda/Panel/models/dbmod"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestLoginValidateTokenAndLogout(t *testing.T) {
	db := newTestDB(t)
	service := svc.NewService(db, time.Hour)

	if err := service.EnsureBootstrapAdmin("admin", "password123"); err != nil {
		t.Fatalf("bootstrap admin: %v", err)
	}

	if _, err := service.Login("admin", "wrong-password"); !errors.Is(err, apperr.ErrInvalidCredentials) {
		t.Fatalf("expected invalid credentials, got %v", err)
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

	if err := service.Logout(session.ID); err != nil {
		t.Fatalf("logout: %v", err)
	}
	if _, _, err := service.ValidateToken(login.Token); !errors.Is(err, apperr.ErrInvalidCredentials) {
		t.Fatalf("expected revoked token to be invalid, got %v", err)
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
