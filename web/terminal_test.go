//go:build linux || darwin

package web

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	authsvc "github.com/KevinZonda/Panel/domain/auth/svc"
	terminalsvc "github.com/KevinZonda/Panel/domain/terminal/svc"
	"github.com/KevinZonda/Panel/lib/passwordx"
	"github.com/KevinZonda/Panel/models"
	"github.com/KevinZonda/Panel/models/dbmod"
	"github.com/coder/websocket"
	"github.com/gin-gonic/gin"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestLocalTerminalAccessAndSession(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "panel.db")), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&dbmod.User{}, &dbmod.AuthSession{}); err != nil {
		t.Fatal(err)
	}
	const password = "terminal-test-password"
	hash, err := passwordx.Hash(password)
	if err != nil {
		t.Fatal(err)
	}
	auth := authsvc.NewService(db, time.Hour)
	if err := auth.EnsureBootstrapAdmin("admin", hash); err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&dbmod.User{
		Username: "viewer", Role: models.RoleViewer, Status: models.UserStatusActive, PasswordHash: hash,
	}).Error; err != nil {
		t.Fatal(err)
	}
	admin, err := auth.Login("admin", password)
	if err != nil {
		t.Fatal(err)
	}
	viewer, err := auth.Login("viewer", password)
	if err != nil {
		t.Fatal(err)
	}

	terminal := terminalsvc.New("")
	engine := gin.New()
	if err := (&Web{authSvc: auth, terminal: terminal}).InitRoutes(engine); err != nil {
		t.Fatal(err)
	}
	server := httptest.NewServer(engine)
	defer server.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	defer func() {
		if err := terminal.Shutdown(ctx); err != nil {
			t.Error(err)
		}
	}()

	for _, tc := range []struct {
		path   string
		token  string
		origin string
		status int
	}{
		{"/ui/terminal", "", "", http.StatusFound},
		{"/ui/terminal", viewer.Token, "", http.StatusForbidden},
		{"/ui/terminal", admin.Token, "", http.StatusOK},
		{"/ui/terminal/ws", viewer.Token, server.URL, http.StatusForbidden},
		{"/ui/terminal/ws", admin.Token, "", http.StatusForbidden},
		{"/ui/terminal/ws", admin.Token, "http://unrelated.example", http.StatusForbidden},
	} {
		req := httptest.NewRequest(http.MethodGet, server.URL+tc.path, nil)
		if tc.token != "" {
			req.AddCookie(&http.Cookie{Name: cookieName, Value: tc.token})
		}
		if tc.origin != "" {
			req.Header.Set("Origin", tc.origin)
		}
		response := httptest.NewRecorder()
		engine.ServeHTTP(response, req)
		if response.Code != tc.status {
			t.Fatalf("%s with origin %q: got %d, want %d", tc.path, tc.origin, response.Code, tc.status)
		}
	}

	if _, err := os.Stat("/bin/bash"); err != nil {
		t.Skipf("local Bash unavailable: %v", err)
	}
	conn, _, err := websocket.Dial(ctx, server.URL+"/ui/terminal/ws", &websocket.DialOptions{
		HTTPHeader: http.Header{"Origin": {server.URL}, "Cookie": {cookieName + "=" + admin.Token}},
	})
	if err != nil {
		t.Fatal(err)
	}
	defer conn.CloseNow()
	if err := conn.Write(ctx, websocket.MessageText, []byte(`{"cols":80,"rows":24}`)); err != nil {
		t.Fatal(err)
	}
	if typ, data, err := conn.Read(ctx); err != nil || typ != websocket.MessageText || string(data) != "ready" {
		t.Fatalf("expected ready message, got %q (%v)", data, err)
	}
	if err := conn.Write(ctx, websocket.MessageText, []byte(`{"cols":110,"rows":35}`)); err != nil {
		t.Fatal(err)
	}
	if err := conn.Write(ctx, websocket.MessageBinary, []byte("printf '\\137TERM_TEST=%s:%s\\n' \"$(id -u)\" \"$(stty size)\"\n")); err != nil {
		t.Fatal(err)
	}
	var output strings.Builder
	want := fmt.Sprintf("_TERM_TEST=%d:35 110", os.Geteuid())
	for !strings.Contains(output.String(), want) {
		typ, data, err := conn.Read(ctx)
		if err != nil {
			t.Fatalf("waiting for %q: %v, output %q", want, err, output.String())
		}
		if typ == websocket.MessageBinary {
			output.Write(data)
		}
	}

	form := url.Values{"csrf_token": {csrfToken(admin.Token)}}
	req := httptest.NewRequest(http.MethodPost, server.URL+"/ui/logout", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.AddCookie(&http.Cookie{Name: cookieName, Value: admin.Token})
	response := httptest.NewRecorder()
	engine.ServeHTTP(response, req)
	if response.Code != http.StatusSeeOther {
		t.Fatalf("logout status = %d", response.Code)
	}
	readCtx, cancelRead := context.WithTimeout(ctx, 2*time.Second)
	defer cancelRead()
	for {
		if _, _, err := conn.Read(readCtx); err != nil {
			if readCtx.Err() != nil {
				t.Fatalf("logout left WebSocket open: %v", err)
			}
			break
		}
	}
}
