package system

import (
	"github.com/KevinZonda/Panel/domain/system/handler"
	"github.com/KevinZonda/Panel/domain/system/svc"
)

type Params struct{}

type Domain struct {
	svc     *svc.Service
	handler *handler.Handler
}

func New(_ Params) (*Domain, error) {
	service := svc.NewService()
	return &Domain{
		svc:     service,
		handler: handler.New(service),
	}, nil
}
