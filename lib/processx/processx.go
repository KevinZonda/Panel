package processx

import (
	"bytes"
	"context"
	"errors"
	"os/exec"
	"time"
)

type Result struct {
	Stdout     string
	Stderr     string
	Output     string
	ExitCode   int
	DurationMS int64
	Err        error
}

type Executor interface {
	Run(ctx context.Context, name string, args ...string) Result
}

type OSExecutor struct{}

func NewOSExecutor() *OSExecutor {
	return &OSExecutor{}
}

func (e *OSExecutor) Run(ctx context.Context, name string, args ...string) Result {
	started := time.Now()
	cmd := exec.CommandContext(ctx, name, args...)

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	exitCode := 0
	if err != nil {
		exitCode = -1
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			exitCode = exitErr.ExitCode()
		}
	}

	out := stdout.String()
	if stderr.Len() > 0 {
		if out != "" {
			out += "\n"
		}
		out += stderr.String()
	}

	return Result{
		Stdout:     stdout.String(),
		Stderr:     stderr.String(),
		Output:     out,
		ExitCode:   exitCode,
		DurationMS: time.Since(started).Milliseconds(),
		Err:        err,
	}
}
