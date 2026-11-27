package web

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	nginxsvc "github.com/KevinZonda/Panel/domain/nginx/svc"
	"github.com/KevinZonda/Panel/models"
	"github.com/KevinZonda/Panel/models/apimod"
	"github.com/gin-gonic/gin"
)

func TestNginxActionRedirectsToStoredResult(t *testing.T) {
	gin.SetMode(gin.TestMode)
	root := t.TempDir()
	service := nginxsvc.NewService(root, root, "nginx.service", models.NginxControlModeSystemd, time.Second, successExecutor{})
	w := &Web{nginxSvc: service, actionResults: newActionResultStore()}

	form := url.Values{"action": {"reload"}}
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/ui/nginx/actions", strings.NewReader(form.Encode()))
	c.Request.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	w.nginxAction(c)
	if c.Writer.Status() != http.StatusSeeOther {
		t.Fatalf("expected status %d, got %d", http.StatusSeeOther, c.Writer.Status())
	}
	location := recorder.Header().Get("Location")
	if !strings.HasPrefix(location, "/ui/nginx/action-results/") {
		t.Fatalf("unexpected redirect location: %q", location)
	}
	token := strings.TrimPrefix(location, "/ui/nginx/action-results/")
	data, ok := w.actionResults.get("nginx", token)
	if !ok {
		t.Fatal("redirect token does not resolve to a stored nginx result")
	}
	result, ok := data["Result"].(*apimod.NginxActionResponse)
	if !ok || result == nil {
		t.Fatalf("stored data does not contain nginx action result: %#v", data)
	}
	if result.Action != "reload" {
		t.Fatalf("unexpected action result: %#v", result)
	}
}
