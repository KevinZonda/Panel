package domain

import (
	"github.com/KevinZonda/Panel/domain/auth"
	"github.com/KevinZonda/Panel/domain/cert"
	"github.com/KevinZonda/Panel/domain/frp"
	"github.com/KevinZonda/Panel/domain/nginx"
	"github.com/KevinZonda/Panel/domain/system"
	"github.com/KevinZonda/Panel/domain/user"
	"github.com/KevinZonda/Panel/models"
	"github.com/KevinZonda/Panel/web"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

func InitRoute(engine *gin.Engine, db *gorm.DB, cfg *models.Config) error {
	api := engine.Group("/api/v1")

	authDomain, err := auth.New(auth.Params{
		DB:                     db,
		TokenTTL:               cfg.TokenTTL,
		BootstrapAdminUsername: cfg.BootstrapAdminUsername,
		BootstrapAdminPassword: cfg.BootstrapAdminPassword,
	})
	if err != nil {
		return err
	}

	userDomain, err := user.New(user.Params{DB: db})
	if err != nil {
		return err
	}

	systemDomain, err := system.New(system.Params{})
	if err != nil {
		return err
	}

	nginxDomain, err := nginx.New(nginx.Params{
		SitesEnabledDir: cfg.NginxSitesEnabledDir,
		BackupDir:       cfg.NginxBackupDir,
		ServiceName:     cfg.NginxServiceName,
		ControlMode:     cfg.NginxControlMode,
		CommandTimeout:  cfg.NginxCommandTimeout,
	})
	if err != nil {
		return err
	}

	frpDomain, err := frp.New(frp.Params{
		FrpsConfigDir:  cfg.FrpsConfigDir,
		FrpcConfigDir:  cfg.FrpcConfigDir,
		BackupDir:      cfg.FrpBackupDir,
		CommandTimeout: cfg.FrpCommandTimeout,
	})
	if err != nil {
		return err
	}

	if err := authDomain.InitHttp(api); err != nil {
		return err
	}

	authn := authDomain.Middleware().AuthRequired()
	admin := authDomain.Middleware().RequireRole(models.RoleAdmin)

	if err := userDomain.InitHttp(api, authn, admin); err != nil {
		return err
	}
	if err := systemDomain.InitHttp(api, authn); err != nil {
		return err
	}
	if err := nginxDomain.InitHttp(api, authn, admin); err != nil {
		return err
	}
	if err := frpDomain.InitHttp(api, authn, admin); err != nil {
		return err
	}

	certDomain, err := cert.New(cert.Params{
		PublicDir:   cfg.CertPublicDir,
		SelfSignDir: cfg.CertSelfSignDir,
	})
	if err != nil {
		return err
	}
	if err := certDomain.InitHttp(api, authn, admin); err != nil {
		return err
	}

	webUI := web.New(
		authDomain.Service(),
		userDomain.Service(),
		systemDomain.Service(),
		nginxDomain.Service(),
		certDomain.Service(),
		frpDomain.Service(),
	)
	return webUI.InitRoutes(engine)
}
