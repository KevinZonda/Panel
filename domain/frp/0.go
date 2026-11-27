package frp

import (
	"time"

	"github.com/KevinZonda/Panel/domain/frp/handler"
	"github.com/KevinZonda/Panel/domain/frp/svc"
	"github.com/KevinZonda/Panel/lib/processx"
)

type Params struct {
	FrpsConfigDir  string
	FrpcConfigDir  string
	BackupDir      string
	CommandTimeout time.Duration
	Executor       processx.Executor
}

type Domain struct {
	svc     *svc.Service
	handler *handler.Handler
}

func New(params Params) (*Domain, error) {
	executor := params.Executor
	if executor == nil {
		executor = processx.NewOSExecutor()
	}
	service := svc.NewService(
		params.FrpsConfigDir,
		params.FrpcConfigDir,
		params.BackupDir,
		params.CommandTimeout,
		executor,
	)
	return &Domain{
		svc:     service,
		handler: handler.New(service),
	}, nil
}

func (d *Domain) Service() *svc.Service {
	return d.svc
}
