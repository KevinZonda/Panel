package shared

import (
	"encoding/json"

	"github.com/KevinZonda/Panel/models"
)

const DefaultDataDir = "/var/lib/kvzd-panel"

func NewInitialConfig() *models.Config {
	cfg := defaultConfig()
	cfg.ListenAddr = "127.0.0.1:8080"
	return cfg
}

func MarshalInitialConfig(cfg *models.Config) ([]byte, error) {
	if err := validateConfig(cfg); err != nil {
		return nil, err
	}
	// Emit only settings selected by the wizard. Other settings use normal defaults.
	return json.MarshalIndent(map[string]any{
		"listen_addr":                   cfg.ListenAddr,
		"database_dsn":                  cfg.DatabaseDSN,
		"bootstrap_admin_username":      cfg.BootstrapAdminUsername,
		"bootstrap_admin_password_hash": cfg.BootstrapAdminPasswordHash,
		"nginx": map[string]string{
			"backup_dir":   cfg.NginxBackupDir,
			"control_mode": cfg.NginxControlMode,
		},
		"frp": map[string]string{
			"backup_dir": cfg.FrpBackupDir,
		},
	}, "", "  ")
}
