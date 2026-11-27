package web

import (
	"context"

	"github.com/KevinZonda/Panel/lib/processx"
)

type successExecutor struct{}

func (successExecutor) Run(context.Context, string, ...string) processx.Result {
	return processx.Result{}
}
