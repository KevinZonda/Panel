package shared

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/KevinZonda/Panel/models"
	"github.com/joho/godotenv"
)

func InitCfg() error {
	_ = godotenv.Load()

	tokenTTLHours := envInt("PANEL_TOKEN_TTL_HOURS", 24)
	if tokenTTLHours <= 0 {
		return fmt.Errorf("PANEL_TOKEN_TTL_HOURS must be positive")
	}

	commandTimeoutSeconds := envInt("PANEL_NGINX_COMMAND_TIMEOUT_SECONDS", 10)
	if commandTimeoutSeconds <= 0 {
		return fmt.Errorf("PANEL_NGINX_COMMAND_TIMEOUT_SECONDS must be positive")
	}

	cfg = &models.Config{
		ListenAddr:             envString("PANEL_LISTEN_ADDR", ":8080"),
		DatabaseDSN:            envString("PANEL_DATABASE_DSN", "file:panel.db?_foreign_keys=on"),
		GinMode:                envString("PANEL_GIN_MODE", "debug"),
		CorsAllowedOrigins:     envStringSlice("PANEL_CORS_ALLOWED_ORIGINS", []string{"*"}),
		TrustedProxies:         envStringSlice("PANEL_TRUSTED_PROXIES", nil),
		TokenTTL:               time.Duration(tokenTTLHours) * time.Hour,
		NginxSitesEnabledDir:   envString("PANEL_NGINX_SITES_ENABLED_DIR", "/etc/nginx/sites-enabled"),
		NginxBackupDir:         envString("PANEL_NGINX_BACKUP_DIR", "data/nginx-backups"),
		NginxServiceName:       envString("PANEL_NGINX_SERVICE_NAME", "nginx"),
		NginxCommandTimeout:    time.Duration(commandTimeoutSeconds) * time.Second,
		BootstrapAdminUsername: envString("PANEL_BOOTSTRAP_ADMIN_USERNAME", "admin"),
		BootstrapAdminPassword: os.Getenv("PANEL_BOOTSTRAP_ADMIN_PASSWORD"),
	}
	return nil
}

func envString(key, fallback string) string {
	if v := strings.TrimSpace(os.Getenv(key)); v != "" {
		return v
	}
	return fallback
}

func envInt(key string, fallback int) int {
	v := strings.TrimSpace(os.Getenv(key))
	if v == "" {
		return fallback
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return fallback
	}
	return n
}

func envStringSlice(key string, fallback []string) []string {
	v := strings.TrimSpace(os.Getenv(key))
	if v == "" {
		return fallback
	}
	parts := strings.Split(v, ",")
	items := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part != "" {
			items = append(items, part)
		}
	}
	if len(items) == 0 {
		return fallback
	}
	return items
}
