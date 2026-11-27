//go:build !linux && !darwin

package svc

import (
	"errors"
	"os"
	"os/exec"
)

var errUnsupported = errors.New("local Bash terminals require Linux or macOS")

func openShell(string, Size) (*exec.Cmd, *os.File, error) {
	return nil, nil, errUnsupported
}

func resizePTY(*os.File, Size) error {
	return errUnsupported
}
