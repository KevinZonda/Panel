package models

import "time"

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
	NginxCommandTimeout    time.Duration
	BootstrapAdminUsername string
	BootstrapAdminPassword string
}
