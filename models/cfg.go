package models

import "time"

const (
	NginxControlModeSystemd = "systemd"
	NginxControlModeBinary  = "binary"
)

type Config struct {
	ListenAddr             string
	DatabaseDSN            string
	GinMode                string
	CorsAllowedOrigins     []string
	TrustedProxies         []string
	TokenTTL               time.Duration
	NginxSitesEnabledDir   string
	NginxBackupDir         string
	NginxServiceName       string
	NginxControlMode       string
	NginxCommandTimeout    time.Duration
	CertPublicDir          string
	CertSelfSignDir        string
	BootstrapAdminUsername string
	BootstrapAdminPassword string
}
