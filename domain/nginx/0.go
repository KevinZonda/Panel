package nginx

import (
	"time"

	"github.com/KevinZonda/Panel/domain/nginx/handler"
	"github.com/KevinZonda/Panel/domain/nginx/svc"
	"github.com/KevinZonda/Panel/lib/processx"
)

type Params struct {
	SitesEnabledDir string
	BackupDir       string
	ServiceName     string
	CommandTimeout  time.Duration
	Executor        processx.Executor
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
		params.SitesEnabledDir,
		params.BackupDir,
		params.ServiceName,
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
