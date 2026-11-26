package shared

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestInitCfgLoadsJSONCConfig(t *testing.T) {
	clearPanelEnv(t)
	configPath := filepath.Join(t.TempDir(), "config.json")
	content := `{
  // comments are allowed
  "listen_addr": "127.0.0.1:19090",
  "database_dsn": "file:/tmp/panel-test.db?_foreign_keys=on",
  "gin_mode": "test",
  "cors_allowed_origins": ["https://panel.example.com"],
  "trusted_proxies": ["127.0.0.1"],
  "token_ttl_hours": 2,
  "bootstrap_admin_username": "root",
  "bootstrap_admin_password": "secret",
  "nginx": {
    "sites_enabled_dir": "/tmp/sites-enabled",
    "backup_dir": "/tmp/backups",
    "service_name": "nginx-test",
    "command_timeout_seconds": 3,
  },
}
`
	if err := os.WriteFile(configPath, []byte(content), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}
	t.Setenv("PANEL_CONFIG", configPath)

	if err := InitCfg(); err != nil {
		t.Fatalf("init config: %v", err)
	}
	if cfg.ListenAddr != "127.0.0.1:19090" {
		t.Fatalf("unexpected listen addr: %q", cfg.ListenAddr)
	}
	if cfg.DatabaseDSN != "file:/tmp/panel-test.db?_foreign_keys=on" {
		t.Fatalf("unexpected database dsn: %q", cfg.DatabaseDSN)
	}
	if cfg.GinMode != "test" {
		t.Fatalf("unexpected gin mode: %q", cfg.GinMode)
	}
	if len(cfg.CorsAllowedOrigins) != 1 || cfg.CorsAllowedOrigins[0] != "https://panel.example.com" {
		t.Fatalf("unexpected cors origins: %#v", cfg.CorsAllowedOrigins)
	}
	if len(cfg.TrustedProxies) != 1 || cfg.TrustedProxies[0] != "127.0.0.1" {
		t.Fatalf("unexpected trusted proxies: %#v", cfg.TrustedProxies)
	}
	if cfg.TokenTTL != 2*time.Hour {
		t.Fatalf("unexpected token ttl: %v", cfg.TokenTTL)
	}
	if cfg.BootstrapAdminUsername != "root" || cfg.BootstrapAdminPassword != "secret" {
		t.Fatalf("unexpected bootstrap config")
	}
	if cfg.NginxSitesEnabledDir != "/tmp/sites-enabled" || cfg.NginxBackupDir != "/tmp/backups" {
		t.Fatalf("unexpected nginx paths: %q %q", cfg.NginxSitesEnabledDir, cfg.NginxBackupDir)
	}
	if cfg.NginxServiceName != "nginx-test" || cfg.NginxCommandTimeout != 3*time.Second {
		t.Fatalf("unexpected nginx service config")
	}
}

func TestEnvOverridesConfigFile(t *testing.T) {
	clearPanelEnv(t)
	configPath := filepath.Join(t.TempDir(), "config.json")
	if err := os.WriteFile(configPath, []byte(`{"listen_addr":":10001","token_ttl_hours":8}`), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}
	t.Setenv("PANEL_CONFIG", configPath)
	t.Setenv("PANEL_LISTEN_ADDR", "127.0.0.1:10002")
	t.Setenv("PANEL_TOKEN_TTL_HOURS", "1")

	if err := InitCfg(); err != nil {
		t.Fatalf("init config: %v", err)
	}
	if cfg.ListenAddr != "127.0.0.1:10002" {
		t.Fatalf("expected env listen addr override, got %q", cfg.ListenAddr)
	}
	if cfg.TokenTTL != time.Hour {
		t.Fatalf("expected env token ttl override, got %v", cfg.TokenTTL)
	}
}

func TestInitCfgRejectsUnknownField(t *testing.T) {
	clearPanelEnv(t)
	configPath := filepath.Join(t.TempDir(), "config.json")
	if err := os.WriteFile(configPath, []byte(`{"unknown_field":true}`), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}
	t.Setenv("PANEL_CONFIG", configPath)

	err := InitCfg()
	if err == nil || !strings.Contains(err.Error(), "unknown field") {
		t.Fatalf("expected unknown field error, got %v", err)
	}
}

func TestInitCfgRejectsInvalidIntegerEnv(t *testing.T) {
	clearPanelEnv(t)
	configPath := filepath.Join(t.TempDir(), "config.json")
	if err := os.WriteFile(configPath, []byte(`{}`), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}
	t.Setenv("PANEL_CONFIG", configPath)
	t.Setenv("PANEL_TOKEN_TTL_HOURS", "not-a-number")

	err := InitCfg()
	if err == nil || !strings.Contains(err.Error(), "PANEL_TOKEN_TTL_HOURS must be an integer") {
		t.Fatalf("expected integer env error, got %v", err)
	}
}

func TestInitCfgRejectsMissingExplicitConfig(t *testing.T) {
	clearPanelEnv(t)
	configPath := filepath.Join(t.TempDir(), "missing.json")
	t.Setenv("PANEL_CONFIG", configPath)

	err := InitCfg()
	if err == nil || !strings.Contains(err.Error(), "read config") {
		t.Fatalf("expected missing explicit config error, got %v", err)
	}
}

func TestDecodeJSONCRejectsMultipleValues(t *testing.T) {
	var fc fileConfig
	err := decodeJSONC([]byte(`{} {}`), &fc)
	if err == nil {
		t.Fatal("expected multiple JSON values to be rejected")
	}
}

func TestEmptyPasswordEnvDoesNotOverrideConfig(t *testing.T) {
	clearPanelEnv(t)
	configPath := filepath.Join(t.TempDir(), "config.json")
	if err := os.WriteFile(configPath, []byte(`{"bootstrap_admin_password":"secret"}`), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}
	t.Setenv("PANEL_CONFIG", configPath)
	t.Setenv("PANEL_BOOTSTRAP_ADMIN_PASSWORD", "")

	if err := InitCfg(); err != nil {
		t.Fatalf("init config: %v", err)
	}
	if cfg.BootstrapAdminPassword != "secret" {
		t.Fatalf("expected blank password env to be ignored, got %q", cfg.BootstrapAdminPassword)
	}
}

func TestConfigExampleParses(t *testing.T) {
	raw, err := os.ReadFile("../config.example.json")
	if err != nil {
		t.Fatalf("read config example: %v", err)
	}
	var fc fileConfig
	if err := decodeJSONC(raw, &fc); err != nil {
		t.Fatalf("decode config example: %v", err)
	}
	cfg := defaultConfig()
	applyFileConfig(cfg, &fc)
	if err := validateConfig(cfg); err != nil {
		t.Fatalf("validate config example: %v", err)
	}
}

func clearPanelEnv(t *testing.T) {
	t.Helper()
	for _, key := range []string{
		"PANEL_CONFIG",
		"PANEL_LISTEN_ADDR",
		"PANEL_DATABASE_DSN",
		"PANEL_GIN_MODE",
		"PANEL_CORS_ALLOWED_ORIGINS",
		"PANEL_TRUSTED_PROXIES",
		"PANEL_TOKEN_TTL_HOURS",
		"PANEL_BOOTSTRAP_ADMIN_USERNAME",
		"PANEL_BOOTSTRAP_ADMIN_PASSWORD",
		"PANEL_NGINX_SITES_ENABLED_DIR",
		"PANEL_NGINX_BACKUP_DIR",
		"PANEL_NGINX_SERVICE_NAME",
		"PANEL_NGINX_CONTROL_MODE",
		"PANEL_CERT_PUBLIC_DIR",
		"PANEL_CERT_SELFSIGN_DIR",
		"PANEL_NGINX_COMMAND_TIMEOUT_SECONDS",
	} {
		t.Setenv(key, "")
	}
}
