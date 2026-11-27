package cert

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/KevinZonda/Panel/lib/authctx"
	"github.com/KevinZonda/Panel/lib/httpx"
	"github.com/KevinZonda/Panel/models"
	"github.com/KevinZonda/Panel/models/dbmod"
	"github.com/gin-gonic/gin"
)

func TestReadCertFilePermissionMatrix(t *testing.T) {
	gin.SetMode(gin.TestMode)
	root := t.TempDir()
	publicDir := filepath.Join(root, "public")
	selfSignDir := filepath.Join(root, "selfsign")
	certDir := filepath.Join(publicDir, "example.com")
	if err := os.MkdirAll(certDir, 0o750); err != nil {
		t.Fatal(err)
	}
	for name, content := range map[string]string{
		"pub":       "public certificate\n",
		"priv":      "private key\n",
		"notes.txt": "operator notes\n",
	} {
		if err := os.WriteFile(filepath.Join(certDir, name), []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
	}

	for _, tc := range []struct {
		name       string
		role       string
		file       string
		wantStatus int
	}{
		{"viewer can read public certificate", models.RoleViewer, "pub", http.StatusOK},
		{"viewer cannot read private key", models.RoleViewer, "priv", http.StatusForbidden},
		{"admin can read private key", models.RoleAdmin, "priv", http.StatusOK},
		{"viewer cannot read unknown files", models.RoleViewer, "notes.txt", http.StatusForbidden},
		{"admin can read unknown files", models.RoleAdmin, "notes.txt", http.StatusOK},
	} {
		t.Run(tc.name, func(t *testing.T) {
			domain, err := New(Params{PublicDir: publicDir, SelfSignDir: selfSignDir})
			if err != nil {
				t.Fatal(err)
			}
			engine := gin.New()
			api := engine.Group("/api/v1")
			authn := func(c *gin.Context) {
				authctx.SetUser(c, &dbmod.User{ID: 1, Username: "tester", Role: tc.role, Status: models.UserStatusActive})
				c.Next()
			}
			admin := func(c *gin.Context) {
				user := authctx.MustUser(c)
				if user == nil || user.Role != models.RoleAdmin {
					httpx.Forbidden(c, "permission denied")
					return
				}
				c.Next()
			}
			if err := domain.InitHttp(api, authn, admin); err != nil {
				t.Fatal(err)
			}

			req := httptest.NewRequest(http.MethodGet, "/api/v1/certs/public/example.com/files/"+tc.file, nil)
			response := httptest.NewRecorder()
			engine.ServeHTTP(response, req)
			if response.Code != tc.wantStatus {
				t.Fatalf("status = %d, want %d, body %s", response.Code, tc.wantStatus, response.Body.String())
			}
		})
	}
}
