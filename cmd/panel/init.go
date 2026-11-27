package main

import (
	"bufio"
	"bytes"
	"errors"
	"flag"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"

	"github.com/KevinZonda/Panel/lib/passwordx"
	"github.com/KevinZonda/Panel/lib/validatex"
	"github.com/KevinZonda/Panel/models"
	"github.com/KevinZonda/Panel/shared"
	"github.com/joho/godotenv"
	"golang.org/x/term"
)

type initWizard struct {
	input        *bufio.Reader
	output       io.Writer
	readPassword func() ([]byte, error)
}

func runInit(args []string, input *os.File, output io.Writer) error {
	flags := flag.NewFlagSet("panel init", flag.ContinueOnError)
	flags.SetOutput(output)
	profile := flags.String("profile", "", "deployment profile (host or docker)")
	configPath := flags.String("config", "", "path to the new configuration file")
	if err := flags.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return nil
		}
		return err
	}
	if flags.NArg() != 0 {
		return fmt.Errorf("unexpected argument %q", flags.Arg(0))
	}
	fd := int(input.Fd())
	if !term.IsTerminal(fd) {
		return fmt.Errorf("panel init requires an interactive terminal")
	}
	_ = godotenv.Load()
	wizard := initWizard{
		input:        bufio.NewReader(input),
		output:       output,
		readPassword: func() ([]byte, error) { return term.ReadPassword(fd) },
	}
	return wizard.run(*profile, *configPath)
}

func (w initWizard) run(profile, configPath string) error {
	var err error
	if profile == "" {
		profile, err = w.choose("Deployment", "host", "host", "docker")
		if err != nil {
			return err
		}
	}
	if profile != "host" && profile != "docker" {
		return fmt.Errorf("invalid deployment profile %q (expected host or docker)", profile)
	}
	docker := profile == "docker"
	if configPath == "" {
		defaultPath := shared.DefaultConfigPath
		if envPath := strings.TrimSpace(os.Getenv("PANEL_CONFIG")); envPath != "" {
			defaultPath = envPath
		}
		configPath, err = w.ask("Configuration file", defaultPath)
		if err != nil {
			return err
		}
	}
	configPath, err = filepath.Abs(configPath)
	if err != nil {
		return fmt.Errorf("resolve configuration path: %w", err)
	}
	if err := checkNewConfigPath(configPath); err != nil {
		return err
	}

	cfg := shared.NewInitialConfig()
	dataDir := shared.DefaultDataDir
	if docker {
		cfg.ListenAddr = ":8080"
		cfg.NginxControlMode = models.NginxControlModeBinary
		fmt.Fprintln(w.output, "Use :PORT to listen on the container interfaces. Docker must publish that port for host access.")
	}
	cfg.ListenAddr, err = w.ask("Panel listen address", cfg.ListenAddr)
	if err != nil {
		return err
	}
	_, port, err := net.SplitHostPort(cfg.ListenAddr)
	listenPort, portErr := strconv.ParseUint(port, 10, 16)
	if err != nil || portErr != nil || listenPort == 0 {
		return fmt.Errorf("listen address must include a numeric port, such as 127.0.0.1:8080 or :8080")
	}
	if !docker {
		dataDir, err = w.configureHost(cfg)
		if err != nil {
			return err
		}
		if dataDir == configPath || strings.HasPrefix(dataDir, configPath+string(os.PathSeparator)) {
			return fmt.Errorf("data directory and its parents must be separate from the configuration file")
		}
	}

	fmt.Fprintln(w.output, "Web Terminal gives Panel administrators access to a Bash shell.")
	terminalChoice, err := w.choose("Enable web Terminal?", "no", "yes", "no", "y", "n")
	if err != nil {
		return err
	}
	terminalSummary := "disabled"
	if terminalChoice == "yes" || terminalChoice == "y" {
		cfg.TerminalEnabled = true
		if docker {
			fmt.Fprintln(w.output, "Use a system account inside the container. The default Panel container runs as root.")
		}
		cfg.TerminalLoginAs, err = w.ask("Terminal system account (blank uses the Panel process account)", "")
		if err != nil {
			return err
		}
		terminalSummary = "enabled (Panel process account)"
		if cfg.TerminalLoginAs != "" {
			terminalSummary = fmt.Sprintf("enabled (%s)", cfg.TerminalLoginAs)
		}
	}

	cfg.BootstrapAdminUsername, err = w.ask("Administrator username", cfg.BootstrapAdminUsername)
	if err != nil {
		return err
	}
	if !validatex.ValidUsername(cfg.BootstrapAdminUsername) {
		return fmt.Errorf("username must contain 1-64 letters, digits, dots, underscores, or hyphens")
	}
	cfg.BootstrapAdminPasswordHash, err = w.passwordHash()
	if err != nil {
		return err
	}
	content, err := shared.MarshalInitialConfig(cfg)
	if err != nil {
		return err
	}

	fmt.Fprintf(w.output, "\nConfiguration file  %s\nListen address      %s\nData directory      %s\nNginx control       %s\nWeb Terminal        %s\nAdministrator       %s\n",
		configPath, cfg.ListenAddr, dataDir, cfg.NginxControlMode, terminalSummary, cfg.BootstrapAdminUsername)
	fmt.Fprintln(w.output, "The administrator credentials apply when starting with an empty database.")
	if err := w.previewDirectories(configPath, dataDir, cfg, docker); err != nil {
		return err
	}
	answer, err := w.choose("Write configuration?", "no", "yes", "no", "y", "n")
	if err != nil {
		return err
	}
	if answer == "no" || answer == "n" {
		fmt.Fprintln(w.output, "Setup cancelled.")
		return nil
	}

	if !docker {
		if err := os.MkdirAll(dataDir, 0o700); err != nil {
			return fmt.Errorf("create data directory: %w", err)
		}
	}
	if err := writeInitialConfig(configPath, content); err != nil {
		return err
	}
	fmt.Fprintf(w.output, "Configuration written to %s\n", configPath)
	switch {
	case docker && configPath == shared.DefaultConfigPath:
		command := "docker compose up --build"
		if listenPort != 8080 {
			command = fmt.Sprintf("PANEL_CONTAINER_PORT=%d %s", listenPort, command)
		}
		fmt.Fprintf(w.output, "Start from the host with %s.\n", command)
	case docker:
		fmt.Fprintln(w.output, "Mount the file into the container and set its PANEL_CONFIG to the mounted path.")
	default:
		fmt.Fprintf(w.output, "Start with PANEL_CONFIG=%s %s\n", shellQuote(configPath), shellQuote(os.Args[0]))
	}
	return nil
}

func (w initWizard) configureHost(cfg *models.Config) (string, error) {
	fmt.Fprintln(w.output, "Who manages Nginx? Choose systemd for a system service or binary for direct control by Panel.")
	mode, err := w.choose("Nginx control", cfg.NginxControlMode,
		models.NginxControlModeSystemd, models.NginxControlModeBinary)
	if err != nil {
		return "", err
	}
	cfg.NginxControlMode = mode
	dir, err := w.ask("Panel data directory", shared.DefaultDataDir)
	if err != nil {
		return "", err
	}
	dir, err = filepath.Abs(dir)
	if err != nil {
		return "", fmt.Errorf("resolve data directory: %w", err)
	}
	if strings.Contains(dir, "?") {
		return "", fmt.Errorf("Panel data directory cannot contain a question mark")
	}
	if dir != shared.DefaultDataDir {
		cfg.DatabaseDSN = filepath.Join(dir, "data.db") + "?_foreign_keys=on"
		cfg.NginxBackupDir = filepath.Join(dir, "nginx-backups")
		cfg.FrpBackupDir = filepath.Join(dir, "frp-backups")
	}
	return dir, nil
}

func (w initWizard) ask(prompt, defaultValue string) (string, error) {
	if defaultValue != "" {
		prompt += " [" + defaultValue + "]"
	}
	fmt.Fprintf(w.output, "%s: ", prompt)
	line, err := w.input.ReadString('\n')
	if err != nil {
		return "", fmt.Errorf("read %s: %w", strings.ToLower(prompt), err)
	}
	value := strings.TrimSpace(line)
	if value == "" {
		return defaultValue, nil
	}
	return value, nil
}

func (w initWizard) choose(prompt, defaultValue string, choices ...string) (string, error) {
	for {
		choice, err := w.ask(prompt+" ("+strings.Join(choices, "/")+")", defaultValue)
		if err != nil {
			return "", err
		}
		choice = strings.ToLower(choice)
		if slices.Contains(choices, choice) {
			return choice, nil
		}
		fmt.Fprintf(w.output, "Choose one of %s.\n", strings.Join(choices, ", "))
	}
}

func (w initWizard) passwordHash() (string, error) {
	fmt.Fprint(w.output, "Administrator password: ")
	password, err := w.readPassword()
	fmt.Fprintln(w.output)
	if err != nil {
		return "", fmt.Errorf("read administrator password: %w", err)
	}
	defer clear(password)
	if err := passwordx.ValidatePassword(string(password)); err != nil {
		return "", fmt.Errorf("administrator password is invalid: %w", err)
	}
	fmt.Fprint(w.output, "Confirm administrator password: ")
	confirmation, err := w.readPassword()
	fmt.Fprintln(w.output)
	if err != nil {
		return "", fmt.Errorf("read password confirmation: %w", err)
	}
	defer clear(confirmation)
	if !bytes.Equal(password, confirmation) {
		return "", fmt.Errorf("administrator passwords do not match")
	}
	return passwordx.Hash(string(password))
}

func shellQuote(value string) string {
	return "'" + strings.ReplaceAll(value, "'", "'\\''") + "'"
}
