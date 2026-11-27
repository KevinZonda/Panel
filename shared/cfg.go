package shared

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/KevinZonda/Panel/lib/passwordx"
	"github.com/KevinZonda/Panel/models"
	"github.com/joho/godotenv"
	"github.com/tailscale/hujson"
)

const DefaultConfigPath = "/etc/kvzd-panel/config.json"

type fileConfig struct {
	ListenAddr                   *string             `json:"listen_addr"`
	DatabaseDSN                  *string             `json:"database_dsn"`
	GinMode                      *string             `json:"gin_mode"`
	CorsAllowedOrigins           []string            `json:"cors_allowed_origins"`
	TrustedProxies               []string            `json:"trusted_proxies"`
	TokenTTLHours                *int                `json:"token_ttl_hours"`
	BootstrapAdminUsername       *string             `json:"bootstrap_admin_username"`
	LegacyBootstrapAdminPassword *string             `json:"bootstrap_admin_password"`
	BootstrapAdminPasswordHash   *string             `json:"bootstrap_admin_password_hash"`
	Nginx                        *fileNginxConfig    `json:"nginx"`
	Frp                          *fileFrpConfig      `json:"frp"`
	Cert                         *fileCertConfig     `json:"cert"`
	Terminal                     *fileTerminalConfig `json:"terminal"`
}

type fileTerminalConfig struct {
	Enabled bool   `json:"enabled"`
	LoginAs string `json:"login_as"`
}

type fileFrpConfig struct {
	FrpsConfigDir         *string `json:"frps_config_dir"`
	FrpcConfigDir         *string `json:"frpc_config_dir"`
	BackupDir             *string `json:"backup_dir"`
	CommandTimeoutSeconds *int    `json:"command_timeout_seconds"`
}

type fileCertConfig struct {
	PublicDir   *string `json:"public_dir"`
	SelfSignDir *string `json:"selfsign_dir"`
}

type fileNginxConfig struct {
	SitesEnabledDir       *string `json:"sites_enabled_dir"`
	BackupDir             *string `json:"backup_dir"`
	ServiceName           *string `json:"service_name"`
	ControlMode           *string `json:"control_mode"`
	CommandTimeoutSeconds *int    `json:"command_timeout_seconds"`
}

func InitCfg() error {
	_ = godotenv.Load()

	loaded := defaultConfig()
	if err := applyConfigFile(loaded); err != nil {
		return err
	}
	if err := applyEnvOverrides(loaded); err != nil {
		return err
	}
	if err := validateConfig(loaded); err != nil {
		return err
	}
	cfg = loaded
	return nil
}

func defaultConfig() *models.Config {
	return &models.Config{
		ListenAddr:                 ":8080",
		DatabaseDSN:                "file:" + DefaultDataDir + "/data.db?_foreign_keys=on",
		GinMode:                    "release",
		CorsAllowedOrigins:         []string{"*"},
		TrustedProxies:             nil,
		TokenTTL:                   24 * time.Hour,
		NginxSitesEnabledDir:       "/etc/nginx/sites-enabled",
		NginxBackupDir:             DefaultDataDir + "/nginx-backups",
		NginxServiceName:           "nginx",
		NginxControlMode:           models.NginxControlModeSystemd,
		FrpsConfigDir:              "/etc/frps",
		FrpcConfigDir:              "/etc/frp",
		FrpBackupDir:               DefaultDataDir + "/frp-backups",
		FrpCommandTimeout:          10 * time.Second,
		CertPublicDir:              "/var/cert",
		CertSelfSignDir:            "/var/cert-self-sign",
		NginxCommandTimeout:        10 * time.Second,
		BootstrapAdminUsername:     "admin",
		BootstrapAdminPasswordHash: "",
	}
}

func applyConfigFile(cfg *models.Config) error {
	path := strings.TrimSpace(os.Getenv("PANEL_CONFIG"))
	explicit := path != ""
	if !explicit {
		path = DefaultConfigPath
	}

	raw, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) && !explicit {
			return nil
		}
		return fmt.Errorf("read config %q: %w", path, err)
	}

	var fc fileConfig
	if err := decodeJSONC(raw, &fc); err != nil {
		return fmt.Errorf("parse config %q: %w", path, err)
	}
	if fc.LegacyBootstrapAdminPassword != nil && strings.TrimSpace(*fc.LegacyBootstrapAdminPassword) != "" {
		return fmt.Errorf("bootstrap_admin_password is no longer supported; use bootstrap_admin_password_hash")
	}
	applyFileConfig(cfg, &fc)
	return nil
}

func decodeJSONC(raw []byte, out any) error {
	standardJSON, err := hujson.Standardize(bytes.Clone(raw))
	if err != nil {
		return err
	}
	decoder := json.NewDecoder(bytes.NewReader(standardJSON))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(out); err != nil {
		return err
	}
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		if err == nil {
			return fmt.Errorf("config contains multiple JSON values")
		}
		return err
	}
	return nil
}

func applyFileConfig(cfg *models.Config, fc *fileConfig) {
	if fc.ListenAddr != nil {
		cfg.ListenAddr = strings.TrimSpace(*fc.ListenAddr)
	}
	if fc.DatabaseDSN != nil {
		cfg.DatabaseDSN = strings.TrimSpace(*fc.DatabaseDSN)
	}
	if fc.GinMode != nil {
		cfg.GinMode = strings.TrimSpace(*fc.GinMode)
	}
	if fc.CorsAllowedOrigins != nil {
		cfg.CorsAllowedOrigins = trimStrings(fc.CorsAllowedOrigins)
	}
	if fc.TrustedProxies != nil {
		cfg.TrustedProxies = trimStrings(fc.TrustedProxies)
	}
	if fc.TokenTTLHours != nil {
		cfg.TokenTTL = time.Duration(*fc.TokenTTLHours) * time.Hour
	}
	if fc.BootstrapAdminUsername != nil {
		cfg.BootstrapAdminUsername = strings.TrimSpace(*fc.BootstrapAdminUsername)
	}
	if fc.BootstrapAdminPasswordHash != nil {
		cfg.BootstrapAdminPasswordHash = strings.TrimSpace(*fc.BootstrapAdminPasswordHash)
	}
	if fc.Terminal != nil {
		cfg.TerminalEnabled = fc.Terminal.Enabled
		cfg.TerminalLoginAs = strings.TrimSpace(fc.Terminal.LoginAs)
	}
	if fc.Cert != nil {
		if fc.Cert.PublicDir != nil {
			cfg.CertPublicDir = strings.TrimSpace(*fc.Cert.PublicDir)
		}
		if fc.Cert.SelfSignDir != nil {
			cfg.CertSelfSignDir = strings.TrimSpace(*fc.Cert.SelfSignDir)
		}
	}
	if fc.Nginx != nil {
		if fc.Nginx.SitesEnabledDir != nil {
			cfg.NginxSitesEnabledDir = strings.TrimSpace(*fc.Nginx.SitesEnabledDir)
		}
		if fc.Nginx.BackupDir != nil {
			cfg.NginxBackupDir = strings.TrimSpace(*fc.Nginx.BackupDir)
		}
		if fc.Nginx.ServiceName != nil {
			cfg.NginxServiceName = strings.TrimSpace(*fc.Nginx.ServiceName)
		}
		if fc.Nginx.ControlMode != nil {
			cfg.NginxControlMode = strings.TrimSpace(*fc.Nginx.ControlMode)
		}
		if fc.Nginx.CommandTimeoutSeconds != nil {
			cfg.NginxCommandTimeout = time.Duration(*fc.Nginx.CommandTimeoutSeconds) * time.Second
		}
	}
	if fc.Frp != nil {
		if fc.Frp.FrpsConfigDir != nil {
			cfg.FrpsConfigDir = strings.TrimSpace(*fc.Frp.FrpsConfigDir)
		}
		if fc.Frp.FrpcConfigDir != nil {
			cfg.FrpcConfigDir = strings.TrimSpace(*fc.Frp.FrpcConfigDir)
		}
		if fc.Frp.BackupDir != nil {
			cfg.FrpBackupDir = strings.TrimSpace(*fc.Frp.BackupDir)
		}
		if fc.Frp.CommandTimeoutSeconds != nil {
			cfg.FrpCommandTimeout = time.Duration(*fc.Frp.CommandTimeoutSeconds) * time.Second
		}
	}
}

func applyEnvOverrides(cfg *models.Config) error {
	if v, ok := envString("PANEL_LISTEN_ADDR"); ok {
		cfg.ListenAddr = v
	}
	if v, ok := envString("PANEL_DATABASE_DSN"); ok {
		cfg.DatabaseDSN = v
	}
	if v, ok := envString("PANEL_GIN_MODE"); ok {
		cfg.GinMode = v
	}
	if v, ok := envStringSlice("PANEL_CORS_ALLOWED_ORIGINS"); ok {
		cfg.CorsAllowedOrigins = v
	}
	if v, ok := envStringSlice("PANEL_TRUSTED_PROXIES"); ok {
		cfg.TrustedProxies = v
	}
	if v, ok, err := envInt("PANEL_TOKEN_TTL_HOURS"); err != nil {
		return err
	} else if ok {
		cfg.TokenTTL = time.Duration(v) * time.Hour
	}
	if v, ok := envString("PANEL_BOOTSTRAP_ADMIN_USERNAME"); ok {
		cfg.BootstrapAdminUsername = v
	}
	if strings.TrimSpace(os.Getenv("PANEL_BOOTSTRAP_ADMIN_PASSWORD")) != "" {
		return fmt.Errorf("PANEL_BOOTSTRAP_ADMIN_PASSWORD is no longer supported; use PANEL_BOOTSTRAP_ADMIN_PASSWORD_HASH")
	}
	if v, ok := envString("PANEL_BOOTSTRAP_ADMIN_PASSWORD_HASH"); ok {
		cfg.BootstrapAdminPasswordHash = v
	}
	if v, ok := envString("PANEL_NGINX_SITES_ENABLED_DIR"); ok {
		cfg.NginxSitesEnabledDir = v
	}
	if v, ok := envString("PANEL_NGINX_BACKUP_DIR"); ok {
		cfg.NginxBackupDir = v
	}
	if v, ok := envString("PANEL_NGINX_SERVICE_NAME"); ok {
		cfg.NginxServiceName = v
	}
	if v, ok := envString("PANEL_NGINX_CONTROL_MODE"); ok {
		cfg.NginxControlMode = v
	}
	if v, ok := envString("PANEL_CERT_PUBLIC_DIR"); ok {
		cfg.CertPublicDir = v
	}
	if v, ok := envString("PANEL_CERT_SELFSIGN_DIR"); ok {
		cfg.CertSelfSignDir = v
	}
	if v, ok, err := envInt("PANEL_NGINX_COMMAND_TIMEOUT_SECONDS"); err != nil {
		return err
	} else if ok {
		cfg.NginxCommandTimeout = time.Duration(v) * time.Second
	}
	if v, ok := envString("PANEL_FRP_FRPS_CONFIG_DIR"); ok {
		cfg.FrpsConfigDir = v
	}
	if v, ok := envString("PANEL_FRP_FRPC_CONFIG_DIR"); ok {
		cfg.FrpcConfigDir = v
	}
	if v, ok := envString("PANEL_FRP_BACKUP_DIR"); ok {
		cfg.FrpBackupDir = v
	}
	if v, ok, err := envInt("PANEL_FRP_COMMAND_TIMEOUT_SECONDS"); err != nil {
		return err
	} else if ok {
		cfg.FrpCommandTimeout = time.Duration(v) * time.Second
	}
	return nil
}

func validateConfig(cfg *models.Config) error {
	if cfg.ListenAddr == "" {
		return fmt.Errorf("listen_addr must not be empty")
	}
	if cfg.DatabaseDSN == "" {
		return fmt.Errorf("database_dsn must not be empty")
	}
	switch cfg.GinMode {
	case "debug", "release", "test":
	default:
		return fmt.Errorf("gin_mode must be one of debug, release, test")
	}
	if cfg.TokenTTL <= 0 {
		return fmt.Errorf("token_ttl_hours must be positive")
	}
	if cfg.NginxSitesEnabledDir == "" {
		return fmt.Errorf("nginx.sites_enabled_dir must not be empty")
	}
	if cfg.NginxBackupDir == "" {
		return fmt.Errorf("nginx.backup_dir must not be empty")
	}
	if cfg.NginxServiceName == "" {
		return fmt.Errorf("nginx.service_name must not be empty")
	}
	switch cfg.NginxControlMode {
	case models.NginxControlModeSystemd, models.NginxControlModeBinary:
	default:
		return fmt.Errorf("nginx.control_mode must be one of systemd, binary")
	}
	if cfg.NginxCommandTimeout <= 0 {
		return fmt.Errorf("nginx.command_timeout_seconds must be positive")
	}
	if cfg.FrpsConfigDir == "" {
		return fmt.Errorf("frp.frps_config_dir must not be empty")
	}
	if cfg.FrpcConfigDir == "" {
		return fmt.Errorf("frp.frpc_config_dir must not be empty")
	}
	if cfg.FrpBackupDir == "" {
		return fmt.Errorf("frp.backup_dir must not be empty")
	}
	if cfg.FrpCommandTimeout <= 0 {
		return fmt.Errorf("frp.command_timeout_seconds must be positive")
	}
	if cfg.CertPublicDir == "" {
		return fmt.Errorf("cert.public_dir must not be empty")
	}
	if cfg.CertSelfSignDir == "" {
		return fmt.Errorf("cert.selfsign_dir must not be empty")
	}
	if cfg.BootstrapAdminPasswordHash != "" {
		if err := passwordx.ValidateHash(cfg.BootstrapAdminPasswordHash); err != nil {
			return fmt.Errorf("bootstrap_admin_password_hash: %w", err)
		}
	}
	return nil
}

func envString(key string) (string, bool) {
	v := strings.TrimSpace(os.Getenv(key))
	return v, v != ""
}

func envInt(key string) (int, bool, error) {
	raw := strings.TrimSpace(os.Getenv(key))
	if raw == "" {
		return 0, false, nil
	}
	n, err := strconv.Atoi(raw)
	if err != nil {
		return 0, true, fmt.Errorf("%s must be an integer: %w", key, err)
	}
	return n, true, nil
}

func envStringSlice(key string) ([]string, bool) {
	v := strings.TrimSpace(os.Getenv(key))
	if v == "" {
		return nil, false
	}
	return trimStrings(strings.Split(v, ",")), true
}

func trimStrings(values []string) []string {
	items := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" {
			items = append(items, value)
		}
	}
	return items
}
