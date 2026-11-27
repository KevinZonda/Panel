package web

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"strings"
	"testing"
	"time"

	authsvc "github.com/KevinZonda/Panel/domain/auth/svc"
	"github.com/KevinZonda/Panel/lib/passwordx"
	"github.com/KevinZonda/Panel/models/dbmod"
	"github.com/gin-gonic/gin"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestLoginWithSessionCookieAbsentFromPageRequest(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "panel.db")), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })
	if err := db.AutoMigrate(&dbmod.User{}, &dbmod.AuthSession{}); err != nil {
		t.Fatal(err)
	}
	const password = "test admin password"
	hash, err := passwordx.Hash(password)
	if err != nil {
		t.Fatal(err)
	}
	auth := authsvc.NewService(db, time.Hour)
	if err := auth.EnsureBootstrapAdmin("admin", hash); err != nil {
		t.Fatal(err)
	}
	engine := gin.New()
	if err := (&Web{authSvc: auth}).InitRoutes(engine); err != nil {
		t.Fatal(err)
	}
	const origin = "https://panel.example.test"
	page := httptest.NewRecorder()
	engine.ServeHTTP(page, httptest.NewRequest(http.MethodGet, origin+"/ui/login", nil))
	if page.Code != http.StatusOK || page.Header().Get("Cache-Control") != "no-store" {
		t.Fatalf("login page response: status = %d, cache control = %q", page.Code, page.Header().Get("Cache-Control"))
	}

	form := url.Values{"username": {"admin"}, "password": {password}}
	post := httptest.NewRequest(http.MethodPost, origin+"/ui/login", strings.NewReader(form.Encode()))
	post.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	post.Header.Set("Origin", origin)
	post.Header.Set("Sec-Fetch-Site", "same-origin")
	post.AddCookie(&http.Cookie{Name: cookieName, Value: "old-session"})
	response := httptest.NewRecorder()
	engine.ServeHTTP(response, post)
	if response.Code != http.StatusSeeOther || response.Header().Get("Location") != "/ui" {
		t.Fatalf("login response: status = %d, location = %q", response.Code, response.Header().Get("Location"))
	}
	for _, cookie := range response.Result().Cookies() {
		if cookie.Name != cookieName {
			continue
		}
		protectedPost := httptest.NewRequest(http.MethodPost, origin+"/ui/password", nil)
		protectedPost.AddCookie(cookie)
		protectedResponse := httptest.NewRecorder()
		engine.ServeHTTP(protectedResponse, protectedPost)
		if protectedResponse.Code != http.StatusForbidden {
			t.Fatalf("protected form without a CSRF token returned %d", protectedResponse.Code)
		}
		return
	}
	t.Fatal("successful login did not set the session cookie")
}

func TestLoginRejectsCrossOriginForm(t *testing.T) {
	engine := gin.New()
	if err := (&Web{}).InitRoutes(engine); err != nil {
		t.Fatal(err)
	}
	post := httptest.NewRequest(http.MethodPost, "https://panel.example.test/ui/login", nil)
	post.Header.Set("Origin", "https://other.example.test")
	post.Header.Set("Sec-Fetch-Site", "same-site")
	response := httptest.NewRecorder()
	engine.ServeHTTP(response, post)
	if response.Code != http.StatusForbidden {
		t.Fatalf("cross-origin login returned %d", response.Code)
	}
}
