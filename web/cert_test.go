package web

import (
	"bytes"
	"strings"
	"testing"

	"github.com/KevinZonda/Panel/models/apimod"
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
