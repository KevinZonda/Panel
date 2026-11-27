package main

import (
	"bufio"
	"bytes"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/KevinZonda/Panel/lib/passwordx"
	"github.com/KevinZonda/Panel/shared"
)

func TestDockerInitDefaultsToSystemConfigPath(t *testing.T) {
	t.Setenv("PANEL_CONFIG", "")
	var output bytes.Buffer
	wizard := initWizard{
		input:  bufio.NewReader(strings.NewReader("")),
		output: &output,
	}
	if err := wizard.run("docker", ""); !errors.Is(err, io.EOF) {
		t.Fatalf("expected end of input at the configuration prompt, got %v", err)
	}
	if !strings.Contains(output.String(), "Configuration file [/etc/kvzd-panel/config.json]") {
		t.Fatalf("unexpected Docker configuration prompt: %q", output.String())
	}
}

func TestInitWizardWritesConfigForStartup(t *testing.T) {
	for _, profile := range []string{"host", "docker"} {
		t.Run(profile, func(t *testing.T) {
			root := t.TempDir()
			path := filepath.Join(root, "etc", "config.json")
			dataDir := filepath.Join(root, "panel data")
			answers := ":9090\nyes\nwww-data\nmaintainer\nyes\n"
			listenAddr, nginxMode := ":9090", "binary"
			terminalEnabled, terminalAccount := true, "www-data"
			if profile == "host" {
				listenAddr, nginxMode = "127.0.0.1:8080", "systemd"
				terminalEnabled, terminalAccount = false, ""
				answers = "\n\n" + dataDir + "\n\nmaintainer\nyes\n"
			}

			const password = "example passphrase"
			var output bytes.Buffer
			wizard := initWizard{
				input:        bufio.NewReader(strings.NewReader(answers)),
				output:       &output,
				readPassword: func() ([]byte, error) { return []byte(password), nil },
			}
			if err := wizard.run(profile, path); err != nil {
				t.Fatalf("run setup: %v", err)
			}

			info, err := os.Stat(path)
			if err != nil {
				t.Fatal(err)
			}
			if info.Mode().Perm() != 0o600 {
				t.Fatalf("config mode = %o, want 600", info.Mode().Perm())
			}
			if strings.Contains(output.String(), password) {
				t.Fatal("setup printed the password")
			}

			t.Setenv("PANEL_CONFIG", path)
			if err := shared.InitCfg(); err != nil {
				t.Fatalf("load generated config: %v", err)
			}
			cfg, err := shared.GetConfig()
			if err != nil {
				t.Fatal(err)
			}
			if cfg.ListenAddr != listenAddr || cfg.NginxControlMode != nginxMode {
				t.Fatalf("unexpected listen address or Nginx mode: %s, %s", cfg.ListenAddr, cfg.NginxControlMode)
			}
			if cfg.TerminalEnabled != terminalEnabled || cfg.TerminalLoginAs != terminalAccount {
				t.Fatalf("unexpected Terminal config: enabled = %t, login_as = %q", cfg.TerminalEnabled, cfg.TerminalLoginAs)
			}
			if cfg.BootstrapAdminUsername != "maintainer" || !passwordx.Compare(cfg.BootstrapAdminPasswordHash, password) {
				t.Fatal("generated config did not preserve the administrator credentials")
			}
			if strings.Contains(output.String(), cfg.BootstrapAdminPasswordHash) {
				t.Fatal("setup printed the password hash")
			}
			if profile == "host" {
				if cfg.DatabaseDSN != filepath.Join(dataDir, "data.db")+"?_foreign_keys=on" {
					t.Fatalf("unexpected database DSN: %q", cfg.DatabaseDSN)
				}
				if info, err := os.Stat(dataDir); err != nil || info.Mode().Perm() != 0o700 {
					t.Fatalf("data directory was not created with mode 700: %v, %v", info, err)
				}
				if _, err := os.Stat(filepath.Join(dataDir, "data.db")); !errors.Is(err, os.ErrNotExist) {
					t.Fatalf("setup opened the database: %v", err)
				}
			}
		})
	}
}

func TestInitWizardCancellationLeavesFilesystemUntouched(t *testing.T) {
	root := t.TempDir()
	dataDir := filepath.Join(root, "data")
	configDir := filepath.Join(root, "etc")
	wizard := initWizard{
		input:        bufio.NewReader(strings.NewReader("\n\n" + dataDir + "\n\n\n\n")),
		output:       &bytes.Buffer{},
		readPassword: func() ([]byte, error) { return []byte("example passphrase"), nil },
	}
	if err := wizard.run("host", filepath.Join(configDir, "config.json")); err != nil {
		t.Fatal(err)
	}
	for _, dir := range []string{dataDir, configDir} {
		if _, err := os.Stat(dir); !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("cancelled setup affected %s: %v", dir, err)
		}
	}
}

func TestWriteInitialConfigPreservesExistingFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	if err := os.WriteFile(path, []byte("existing config"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := writeInitialConfig(path, []byte("new config")); !errors.Is(err, os.ErrExist) {
		t.Fatalf("expected path collision, got %v", err)
	}
	content, err := os.ReadFile(path)
	if err != nil || string(content) != "existing config" {
		t.Fatalf("existing config was modified: %q, %v", content, err)
	}
}
