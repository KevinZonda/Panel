package user

import (
	"github.com/KevinZonda/Panel/domain/user/handler"
	"github.com/KevinZonda/Panel/domain/user/svc"
	"gorm.io/gorm"
)

type Params struct {
	DB *gorm.DB
}

type Domain struct {
	svc     *svc.Service
	handler *handler.Handler
}

func New(params Params) (*Domain, error) {
	service := svc.NewService(params.DB)
	return &Domain{
		svc:     service,
		handler: handler.New(service),
	}, nil
}
