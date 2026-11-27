package web

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	frpsvc "github.com/KevinZonda/Panel/domain/frp/svc"
	"github.com/KevinZonda/Panel/models/apimod"
	"github.com/gin-gonic/gin"
)

func TestFrpServiceActionRedirectsToStoredResult(t *testing.T) {
	gin.SetMode(gin.TestMode)
	root := t.TempDir()
	service := frpsvc.NewService(root, root, root, time.Second, successExecutor{})
	w := &Web{frpSvc: service, actionResults: newActionResultStore()}

	form := url.Values{"action": {"restart"}}
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/ui/frp/services/frps-academic.service/actions", strings.NewReader(form.Encode()))
	c.Request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	c.Params = gin.Params{{Key: "name", Value: "frps-academic.service"}}

	w.frpServiceAction(c)
	if c.Writer.Status() != http.StatusSeeOther {
		t.Fatalf("expected status %d, got %d", http.StatusSeeOther, c.Writer.Status())
	}
	location := recorder.Header().Get("Location")
	if !strings.HasPrefix(location, "/ui/frp/action-results/") {
		t.Fatalf("unexpected redirect location: %q", location)
	}
	token := strings.TrimPrefix(location, "/ui/frp/action-results/")
	data, ok := w.actionResults.get("frp", token)
	if !ok {
		t.Fatal("redirect token does not resolve to a stored frp result")
	}
	result, ok := data["Result"].(*apimod.FrpServiceActionResponse)
	if !ok || result == nil {
		t.Fatalf("stored data does not contain frp action result: %#v", data)
	}
	if result.Service != "frps-academic.service" || result.Action != "restart" {
		t.Fatalf("unexpected action result: %#v", result)
	}
}

func TestFrpRoutesRegister(t *testing.T) {
	gin.SetMode(gin.TestMode)
	if err := (&Web{}).InitRoutes(gin.New()); err != nil {
		t.Fatalf("init routes: %v", err)
	}
}

func TestFrpTemplatesRender(t *testing.T) {
	tmpl, err := loadTemplates()
	if err != nil {
		t.Fatalf("load templates: %v", err)
	}
	user := map[string]string{"Username": "admin", "Role": "admin"}
	pid := 1234
	service := apimod.FrpServiceStatus{
		Name:          "frps-academic.service",
		Type:          apimod.FrpTypeServer,
		Description:   "academic frp server",
		ActiveState:   "active",
		SubState:      "running",
		UnitFileState: "enabled",
		MainPID:       &pid,
		Running:       true,
		ConfigType:    apimod.FrpTypeServer,
		ConfigName:    "frps-academic.toml",
	}
	config := &apimod.FrpConfigFile{
		FrpConfigSummary: apimod.FrpConfigSummary{
			Type:        apimod.FrpTypeServer,
			Name:        "frps-academic.toml",
			ServiceName: "frps-academic.service",
			SizeBytes:   32,
			ModifiedAt:  time.Now(),
			SHA256:      "abc123",
		},
		Content: "bindPort = 7000\n",
	}

	t.Run("index", func(t *testing.T) {
		var out bytes.Buffer
		err := tmpl.ExecuteTemplate(&out, "frp.html", map[string]any{
			"Title":     "frpc/frps Management",
			"User":      user,
			"CSRFToken": "csrf",
			"Services":  []apimod.FrpServiceStatus{service},
			"Configs":   []apimod.FrpConfigSummary{config.FrpConfigSummary},
		})
		if err != nil {
			t.Fatalf("render frp index: %v", err)
		}
		html := out.String()
		for _, want := range []string{"frps-academic.service", "frps-academic.toml", "Create config", `value="csrf"`} {
			if !strings.Contains(html, want) {
				t.Fatalf("frp index missing %q", want)
			}
		}
	})

	t.Run("viewer does not get config links", func(t *testing.T) {
		var out bytes.Buffer
		err := tmpl.ExecuteTemplate(&out, "frp.html", map[string]any{
			"Title":     "frpc/frps Management",
			"User":      map[string]string{"Username": "viewer", "Role": "viewer"},
			"CSRFToken": "csrf",
			"Services":  []apimod.FrpServiceStatus{service},
		})
		if err != nil {
			t.Fatalf("render viewer frp index: %v", err)
		}
		html := out.String()
		if strings.Contains(html, "/ui/frp/configs/frps/frps-academic.toml") {
			t.Fatal("viewer sees an admin-only frp config link")
		}
		if !strings.Contains(html, "frps-academic.toml") {
			t.Fatal("viewer does not see config name as plain text")
		}
		if strings.Contains(html, "<th>Actions</th>") || strings.Contains(html, "Create config") {
			t.Fatal("viewer sees admin-only frp controls")
		}
	})

	t.Run("config", func(t *testing.T) {
		var out bytes.Buffer
		err := tmpl.ExecuteTemplate(&out, "frp_config.html", map[string]any{
			"Title":     "frp Config",
			"User":      user,
			"CSRFToken": "csrf",
			"Config":    config,
			"Content":   config.Content,
		})
		if err != nil {
			t.Fatalf("render frp config: %v", err)
		}
		html := out.String()
		for _, want := range []string{"frps-academic.toml", "bindPort = 7000", `value="csrf"`} {
			if !strings.Contains(html, want) {
				t.Fatalf("frp config missing %q", want)
			}
		}
	})

	t.Run("action result", func(t *testing.T) {
		var out bytes.Buffer
		err := tmpl.ExecuteTemplate(&out, "frp_action_result.html", map[string]any{
			"Title": "frp Service Action",
			"User":  user,
			"Result": &apimod.FrpServiceActionResponse{
				Service: service.Name,
				Action:  "restart",
				OK:      true,
				Status:  service,
			},
		})
		if err != nil {
			t.Fatalf("render frp action result: %v", err)
		}
		if !strings.Contains(out.String(), "frps-academic.service") {
			t.Fatal("frp action result missing service")
		}
	})
}
