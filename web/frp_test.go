package web

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"github.com/KevinZonda/Panel/models/apimod"
	"github.com/gin-gonic/gin"
)

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
