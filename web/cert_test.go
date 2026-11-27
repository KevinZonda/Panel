package web

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"

	certsvc "github.com/KevinZonda/Panel/domain/cert/svc"
	"github.com/KevinZonda/Panel/lib/authctx"
	"github.com/KevinZonda/Panel/models"
	"github.com/KevinZonda/Panel/models/apimod"
	"github.com/KevinZonda/Panel/models/dbmod"
	"github.com/gin-gonic/gin"
)

func TestCertReplaceFormsIncludeCSRFToken(t *testing.T) {
	tmpl, err := loadTemplates()
	if err != nil {
		t.Fatalf("load templates: %v", err)
	}

	cert := &apimod.CertDetail{
		CertSummary: apimod.CertSummary{
			Name: "docker-test.com",
			Type: apimod.CertTypeSelfSign,
		},
		Files: []apimod.CertFileInfo{{Name: "server.crt"}},
	}
	data := map[string]any{
		"Title":     "Cert docker-test.com",
		"Cert":      cert,
		"User":      map[string]string{"Username": "admin", "Role": "admin"},
		"CSRFToken": "test-csrf-token",
	}

	var out bytes.Buffer
	if err := tmpl.ExecuteTemplate(&out, "cert.html", data); err != nil {
		t.Fatalf("render cert page: %v", err)
	}
	html := out.String()
	if !strings.Contains(html, `value="test-csrf-token"`) {
		t.Fatalf("replace form does not include page CSRF token")
	}
	if !strings.Contains(html, "Replace server.crt") {
		t.Fatalf("replace form did not render")
	}
}

func TestCertFormErrorRendersOnce(t *testing.T) {
	tmpl, err := loadTemplates()
	if err != nil {
		t.Fatalf("load templates: %v", err)
	}

	for _, page := range []string{"cert_upload.html", "cert_generate.html"} {
		var out bytes.Buffer
		data := map[string]any{
			"Title":   "Certificate form",
			"Error":   "validation failed",
			"Subject": defaultSubject(),
			"Days":    30,
		}
		if err := tmpl.ExecuteTemplate(&out, page, data); err != nil {
			t.Fatalf("render %s: %v", page, err)
		}
		if count := strings.Count(out.String(), "Error:"); count != 1 {
			t.Fatalf("%s renders error %d times, want once", page, count)
		}
	}
}

func TestUploadPublicDoesNotRefillRejectedPrivateKey(t *testing.T) {
	gin.SetMode(gin.TestMode)
	tmpl, err := loadTemplates()
	if err != nil {
		t.Fatalf("load templates: %v", err)
	}
	root := t.TempDir()
	web := &Web{certSvc: certsvc.NewService(filepath.Join(root, "public"), filepath.Join(root, "selfsign"))}
	engine := gin.New()
	engine.SetHTMLTemplate(tmpl)
	engine.POST("/ui/certs/public", func(c *gin.Context) {
		authctx.SetUser(c, &dbmod.User{ID: 1, Username: "admin", Role: models.RoleAdmin})
		web.uploadPublic(c)
	})

	const privateKeyMarker = "REJECTED-PRIVATE-KEY-MATERIAL"
	form := url.Values{
		"name": {"example.com"},
		"pub":  {"not a certificate"},
		"priv": {"-----BEGIN PRIVATE KEY-----\n" + privateKeyMarker + "\n-----END PRIVATE KEY-----"},
	}
	req := httptest.NewRequest(http.MethodPost, "/ui/certs/public", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	response := httptest.NewRecorder()
	engine.ServeHTTP(response, req)
	if response.Code != http.StatusOK {
		t.Fatalf("upload validation response status = %d, body %s", response.Code, response.Body.String())
	}
	if strings.Contains(response.Body.String(), privateKeyMarker) {
		t.Fatal("rejected private key was rendered back into the form")
	}
	if !strings.Contains(response.Body.String(), "not a certificate") {
		t.Fatal("non-secret certificate input should still be restored")
	}
}

func TestCertPageDoesNotExposePrivateKeyToViewer(t *testing.T) {
	gin.SetMode(gin.TestMode)
	tmpl, err := loadTemplates()
	if err != nil {
		t.Fatalf("load templates: %v", err)
	}
	root := t.TempDir()
	publicDir := filepath.Join(root, "public")
	certDir := filepath.Join(publicDir, "example.com")
	if err := os.MkdirAll(certDir, 0o750); err != nil {
		t.Fatal(err)
	}
	const privateKeyMarker = "VIEWER-MUST-NOT-SEE-THIS-KEY"
	if err := os.WriteFile(filepath.Join(certDir, "pub"), []byte("not a certificate"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(certDir, "priv"), []byte(privateKeyMarker), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(certDir, "notes.txt"), []byte("operator notes"), 0o600); err != nil {
		t.Fatal(err)
	}

	web := &Web{certSvc: certsvc.NewService(publicDir, filepath.Join(root, "selfsign"))}
	engine := gin.New()
	engine.SetHTMLTemplate(tmpl)
	engine.GET("/ui/certs/public/:name", func(c *gin.Context) {
		authctx.SetUser(c, &dbmod.User{ID: 2, Username: "viewer", Role: models.RoleViewer})
		c.Set("certType", "public")
		web.certPage(c)
	})

	req := httptest.NewRequest(http.MethodGet, "/ui/certs/public/example.com?file=priv", nil)
	response := httptest.NewRecorder()
	engine.ServeHTTP(response, req)
	if response.Code != http.StatusOK {
		t.Fatalf("cert page status = %d", response.Code)
	}
	if strings.Contains(response.Body.String(), privateKeyMarker) {
		t.Fatal("viewer was able to read private key through the UI")
	}
	if !strings.Contains(response.Body.String(), "admin role required") {
		t.Fatal("expected an explicit server-side permission error")
	}
	if strings.Contains(response.Body.String(), "?file=notes.txt") {
		t.Fatal("viewer was offered a link to an unknown certificate file")
	}
	if !strings.Contains(response.Body.String(), "?file=pub") {
		t.Fatal("viewer should still be offered the public certificate link")
	}
}
