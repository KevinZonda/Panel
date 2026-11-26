package cert

import (
	"github.com/KevinZonda/Panel/domain/cert/handler"
	"github.com/KevinZonda/Panel/domain/cert/svc"
)

type Params struct {
	PublicDir   string
	SelfSignDir string
}

type Domain struct {
	svc     *svc.Service
	handler *handler.Handler
}

func New(params Params) (*Domain, error) {
	service := svc.NewService(params.PublicDir, params.SelfSignDir)
	return &Domain{
		svc:     service,
		handler: handler.New(service),
	}, nil
}

func (d *Domain) Service() *svc.Service {
	return d.svc
}
