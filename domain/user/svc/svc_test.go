package svc_test

import (
	"errors"
	"path/filepath"
	"testing"

	"github.com/KevinZonda/Panel/domain/user/svc"
	"github.com/KevinZonda/Panel/lib/apperr"
	"github.com/KevinZonda/Panel/lib/passwordx"
	"github.com/KevinZonda/Panel/models"
	"github.com/KevinZonda/Panel/models/apimod"
	"github.com/KevinZonda/Panel/models/dbmod"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestLastAdminAndDeleteProtection(t *testing.T) {
	db := newTestDB(t)
	service := svc.NewService(db)

	hash, err := passwordx.Hash("password123")
	if err != nil {
		t.Fatalf("hash password: %v", err)
	}
	admin := &dbmod.User{
		Username:     "admin",
		PasswordHash: hash,
		Role:         models.RoleAdmin,
		Status:       models.UserStatusActive,
	}
	if err := db.Create(admin).Error; err != nil {
		t.Fatalf("create admin: %v", err)
	}

	viewerRole := models.RoleViewer
	if _, err := service.UpdateUser(admin.ID, admin.ID, apimod.UpdateUserRequest{Role: &viewerRole}); !errors.Is(err, apperr.ErrConflict) {
		t.Fatalf("expected self demotion conflict, got %v", err)
	}

	viewer, err := service.CreateUser(apimod.CreateUserRequest{
		Username: "viewer",
		Password: "password123",
		Role:     models.RoleViewer,
		Status:   models.UserStatusActive,
	})
	if err != nil {
		t.Fatalf("create viewer: %v", err)
	}

	if err := service.DeleteUser(admin.ID, viewer.ID); err != nil {
		t.Fatalf("delete viewer: %v", err)
	}
	if _, err := service.GetUser(viewer.ID); !errors.Is(err, apperr.ErrNotFound) {
		t.Fatalf("expected viewer to be deleted, got %v", err)
	}
	if err := service.DeleteUser(admin.ID, admin.ID); !errors.Is(err, apperr.ErrConflict) {
		t.Fatalf("expected self delete conflict, got %v", err)
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
