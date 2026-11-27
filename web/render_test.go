package web

import (
	"bytes"
	"regexp"
	"strings"
	"testing"
)

func TestFooterRendersPanelNameTimeAndBuildInfo(t *testing.T) {
	tmpl, err := loadTemplates()
	if err != nil {
		t.Fatalf("load templates: %v", err)
	}

	var out bytes.Buffer
	if err := tmpl.ExecuteTemplate(&out, "login.html", map[string]any{"Title": "Login"}); err != nil {
		t.Fatalf("render login page: %v", err)
	}
	html := out.String()

	footer := regexp.MustCompile(`<p><span>KevinZonda Panel, \d{4}-\d{2}-\d{2} \d{2}:\d{2}:\d{2}, <i>\(go=go[^<]+builder=[^<]+target=[^<]+\)</i></span></p>`)
	if !footer.MatchString(html) {
		t.Fatalf("footer does not include panel name, current time, and italic build information: %s", html)
	}
	if strings.Contains(html, "<small>kvzd-panel</small>") {
		t.Fatalf("old footer is still rendered")
	}
}
