package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/KevinZonda/Panel/models"
	"golang.org/x/sys/unix"
)

func checkNewConfigPath(path string) error {
	_, err := os.Lstat(path)
	switch {
	case err == nil:
		return fmt.Errorf("configuration %s already exists: %w", path, os.ErrExist)
	case errors.Is(err, os.ErrNotExist):
		return nil
	default:
		return fmt.Errorf("check configuration path: %w", err)
	}
}

func (w initWizard) previewDirectories(configPath, dataDir string, cfg *models.Config, docker bool) error {
	required := []string{filepath.Dir(configPath)}
	if !docker && dataDir != required[0] {
		required = append(required, dataDir)
	}
	for _, path := range required {
		exists, err := checkDirectoryAccess(path, unix.W_OK|unix.X_OK)
		if err != nil {
			return fmt.Errorf("cannot prepare directory %s: %w", path, err)
		}
		if !exists {
			fmt.Fprintf(w.output, "Will create directory %s\n", path)
		}
	}
	if docker {
		fmt.Fprintln(w.output, "The Docker preset uses directories and volumes inside the container.")
		return nil
	}

	paths := []struct {
		label string
		path  string
	}{
		{"Nginx sites", cfg.NginxSitesEnabledDir},
		{"FRP server configs", cfg.FrpsConfigDir},
		{"FRP client configs", cfg.FrpcConfigDir},
		{"Public certificates", cfg.CertPublicDir},
		{"Self-signed certificates", cfg.CertSelfSignDir},
	}
	for _, item := range paths {
		exists, err := checkDirectoryAccess(item.path, unix.R_OK|unix.W_OK|unix.X_OK)
		switch {
		case err != nil:
			fmt.Fprintf(w.output, "%s directory %s needs attention (%v)\n", item.label, item.path, err)
		case !exists:
			fmt.Fprintf(w.output, "%s directory %s is missing\n", item.label, item.path)
		}
	}
	return nil
}

// Check the closest existing parent when a directory is yet to be created.
func checkDirectoryAccess(path string, mode uint32) (bool, error) {
	for candidate := path; ; candidate = filepath.Dir(candidate) {
		info, err := os.Stat(candidate)
		if errors.Is(err, os.ErrNotExist) && candidate != filepath.Dir(candidate) {
			continue
		}
		if err != nil {
			return false, err
		}
		if !info.IsDir() {
			return false, fmt.Errorf("%s is not a directory", candidate)
		}
		if err := unix.Access(candidate, mode); err != nil {
			return false, fmt.Errorf("access %s as the current user: %w", candidate, err)
		}
		return candidate == path, nil
	}
}

func writeInitialConfig(path string, content []byte) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return fmt.Errorf("create configuration directory: %w", err)
	}
	file, err := os.CreateTemp(dir, ".panel-config-*")
	if err != nil {
		return fmt.Errorf("create temporary configuration: %w", err)
	}
	defer os.Remove(file.Name())
	defer file.Close()
	if err := file.Chmod(0o600); err != nil {
		return fmt.Errorf("set configuration permissions: %w", err)
	}
	if _, err := file.Write(append(content, '\n')); err != nil {
		return fmt.Errorf("write configuration: %w", err)
	}
	if err := file.Sync(); err != nil {
		return fmt.Errorf("sync configuration: %w", err)
	}
	if err := file.Close(); err != nil {
		return fmt.Errorf("close configuration: %w", err)
	}
	// Publish the complete file without replacing any existing config or symlink.
	if err := os.Link(file.Name(), path); err != nil {
		return fmt.Errorf("publish configuration: %w", err)
	}
	return nil
}
