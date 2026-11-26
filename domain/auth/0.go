package auth

import (
	"time"

	"github.com/KevinZonda/Panel/domain/auth/handler"
	"github.com/KevinZonda/Panel/domain/auth/middleware"
	"github.com/KevinZonda/Panel/domain/auth/svc"
	"gorm.io/gorm"
)

type Params struct {
	DB                     *gorm.DB
	TokenTTL               time.Duration
	BootstrapAdminUsername string
	BootstrapAdminPassword string
}

type Domain struct {
	svc     *svc.Service
	handler *handler.Handler
	mw      *middleware.Middleware
}

func New(params Params) (*Domain, error) {
	service := svc.NewService(params.DB, params.TokenTTL)
	if err := service.EnsureBootstrapAdmin(params.BootstrapAdminUsername, params.BootstrapAdminPassword); err != nil {
		return nil, err
	}
	return &Domain{
		svc:     service,
		handler: handler.New(service),
		mw:      middleware.New(service),
	}, nil
}

func (d *Domain) Middleware() *middleware.Middleware {
	return d.mw
}
