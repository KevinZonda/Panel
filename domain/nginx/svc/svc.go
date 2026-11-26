package svc

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/KevinZonda/Panel/domain/nginx/dao"
	"github.com/KevinZonda/Panel/lib/apperr"
	"github.com/KevinZonda/Panel/lib/processx"
	"github.com/KevinZonda/Panel/models"
	"github.com/KevinZonda/Panel/models/apimod"
)

const (
	maxSiteContentBytes = 1024 * 1024
	maxCommandOutputLen = 4096
)

type Service struct {
	sitesDir    string
	backupDir   string
	serviceName string
	controlMode string
	timeout     time.Duration
	executor    processx.Executor
	mu          sync.Mutex
}

func NewService(sitesDir, backupDir, serviceName, controlMode string, timeout time.Duration, executor processx.Executor) *Service {
	if controlMode == "" {
		controlMode = models.NginxControlModeSystemd
	}
	return &Service{
		sitesDir:    sitesDir,
		backupDir:   backupDir,
		serviceName: serviceName,
		controlMode: controlMode,
		timeout:     timeout,
		executor:    executor,
	}
}

func (s *Service) Status(ctx context.Context) (*apimod.NginxStatus, error) {
	versionResult := s.run(ctx, "nginx", "-v")
	installed := versionResult.Err == nil && versionResult.ExitCode == 0
	version := parseNginxVersion(resultOutput(versionResult))

	pid, processRunning := s.detectProcess(ctx)
	manager, activeState, subState := s.detectService(ctx)
	running := processRunning || (activeState != nil && *activeState == "active")

	return &apimod.NginxStatus{
		Installed: installed,
		Running:   running,
		PID:       pid,
		Version:   version,
		Service: apimod.NginxServiceStatus{
			Manager:     manager,
			ActiveState: activeState,
			SubState:    subState,
		},
		ConfigTest: s.TestConfig(ctx),
		CheckedAt:  time.Now(),
	}, nil
}

func (s *Service) TestConfig(ctx context.Context) apimod.NginxConfigTestResult {
	result := s.run(ctx, "nginx", "-t")
	return apimod.NginxConfigTestResult{
		OK:         result.Err == nil && result.ExitCode == 0,
		ExitCode:   result.ExitCode,
		Output:     resultOutput(result),
		DurationMS: result.DurationMS,
		CheckedAt:  time.Now(),
	}
}

func (s *Service) Action(ctx context.Context, action string) (*apimod.NginxActionResponse, error) {
	switch action {
	case "start", "stop", "reload", "restart":
	default:
		return nil, apperr.Validationf("unsupported nginx action")
	}

	s.mu.Lock()
	var result processx.Result
	if s.controlMode == models.NginxControlModeBinary {
		result = s.runBinaryAction(ctx, action)
	} else {
		result = s.run(ctx, "systemctl", action, s.serviceName)
	}
	s.mu.Unlock()

	pid, running := s.detectProcess(ctx)
	installed := false
	versionResult := s.run(ctx, "nginx", "-v")
	if versionResult.Err == nil && versionResult.ExitCode == 0 {
		installed = true
	}

	return &apimod.NginxActionResponse{
		Action:     action,
		OK:         result.Err == nil && result.ExitCode == 0,
		ExitCode:   result.ExitCode,
		Output:     resultOutput(result),
		DurationMS: result.DurationMS,
		FinishedAt: time.Now(),
		Status: apimod.NginxActionStatus{
			Installed: installed,
			Running:   running,
			PID:       pid,
		},
	}, nil
}

func (s *Service) ListSites() (*apimod.NginxSiteList, error) {
	items, err := dao.ListSites(s.sitesDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, apperr.Dependencyf("nginx sites-enabled directory does not exist")
		}
		return nil, err
	}
	return &apimod.NginxSiteList{Items: items}, nil
}

func (s *Service) GetSite(name string) (*apimod.NginxSite, error) {
	if err := validateSiteName(name); err != nil {
		return nil, err
	}
	site, err := dao.ReadSite(s.sitesDir, name)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, apperr.NotFoundf("nginx site not found")
		}
		return nil, err
	}
	return site, nil
}

func (s *Service) CreateSite(ctx context.Context, name, content string, validate bool) (*apimod.NginxSiteMutationResult, error) {
	if err := validateSiteInput(name, content); err != nil {
		return nil, err
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	exists, err := dao.SiteExists(s.sitesDir, name)
	if err != nil {
		return nil, err
	}
	if exists {
		return nil, apperr.Conflictf("nginx site already exists")
	}

	summary, err := dao.WriteSite(s.sitesDir, name, content)
	if err != nil {
		return nil, apperr.Dependencyf("write nginx site failed: %v", err)
	}

	validation := apimod.NginxValidationResult{OK: true, Output: "validation skipped"}
	if validate {
		validation, err = s.validateWrittenConfig(ctx, name, false, "")
		if err != nil {
			return nil, err
		}
	}

	return mutationResult(summary, validation), nil
}

func (s *Service) UpdateSite(ctx context.Context, name, content string, validate bool, expectedSHA256 string) (*apimod.NginxSiteMutationResult, error) {
	if err := validateSiteInput(name, content); err != nil {
		return nil, err
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	oldSite, err := dao.ReadSite(s.sitesDir, name)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, apperr.NotFoundf("nginx site not found")
		}
		return nil, err
	}
	if expectedSHA256 != "" && expectedSHA256 != oldSite.SHA256 {
		return nil, apperr.Conflictf("nginx site was modified by another writer")
	}
	if _, err := dao.BackupSite(s.backupDir, name, []byte(oldSite.Content)); err != nil {
		return nil, apperr.Dependencyf("backup nginx site failed: %v", err)
	}

	summary, err := dao.WriteSite(s.sitesDir, name, content)
	if err != nil {
		return nil, apperr.Dependencyf("write nginx site failed: %v", err)
	}

	validation := apimod.NginxValidationResult{OK: true, Output: "validation skipped"}
	if validate {
		validation, err = s.validateWrittenConfig(ctx, name, true, oldSite.Content)
		if err != nil {
			return nil, err
		}
	}

	return mutationResult(summary, validation), nil
}

func (s *Service) DeleteSite(name string) error {
	if err := validateSiteName(name); err != nil {
		return err
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	oldSite, err := dao.ReadSite(s.sitesDir, name)
	if err != nil {
		if os.IsNotExist(err) {
			return apperr.NotFoundf("nginx site not found")
		}
		return err
	}
	if _, err := dao.BackupSite(s.backupDir, name, []byte(oldSite.Content)); err != nil {
		return apperr.Dependencyf("backup nginx site failed: %v", err)
	}
	if err := dao.DeleteSite(s.sitesDir, name); err != nil {
		if os.IsNotExist(err) {
			return apperr.NotFoundf("nginx site not found")
		}
		return err
	}
	return nil
}

func (s *Service) validateWrittenConfig(ctx context.Context, name string, hadOld bool, oldContent string) (apimod.NginxValidationResult, error) {
	test := s.TestConfig(ctx)
	validation := apimod.NginxValidationResult{OK: test.OK, Output: test.Output}
	if test.OK {
		return validation, nil
	}

	var rollbackErr error
	if hadOld {
		_, rollbackErr = dao.WriteSite(s.sitesDir, name, oldContent)
	} else {
		rollbackErr = dao.DeleteSite(s.sitesDir, name)
		if os.IsNotExist(rollbackErr) {
			rollbackErr = nil
		}
	}

	if rollbackErr != nil {
		return validation, apperr.Validationf("nginx config validation failed and rollback failed: %v", rollbackErr)
	}
	return validation, apperr.Validationf("nginx config validation failed: %s", test.Output)
}

func (s *Service) run(ctx context.Context, name string, args ...string) processx.Result {
	timeout := s.timeout
	if timeout <= 0 {
		timeout = 10 * time.Second
	}
	cmdCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	return s.executor.Run(cmdCtx, name, args...)
}

// runBinaryAction controls nginx through its own signals, without any init
// system. This supports environments like containers, Alpine, or BSD hosts.
func (s *Service) runBinaryAction(ctx context.Context, action string) processx.Result {
	switch action {
	case "start":
		return s.run(ctx, "nginx")
	case "stop":
		result := s.run(ctx, "nginx", "-s", "quit")
		// quit is asynchronous; wait for the master process to exit so the
		// reported status matches systemctl stop semantics.
		if result.Err == nil && result.ExitCode == 0 {
			s.waitForProcessExit(ctx)
		}
		return result
	case "reload":
		return s.run(ctx, "nginx", "-s", "reload")
	case "restart":
		return s.runBinaryRestart(ctx)
	}
	return processx.Result{ExitCode: -1, Err: fmt.Errorf("unsupported nginx action %q", action)}
}

// nginx has no restart signal: stop gracefully, wait for the master process
// to exit (quit is asynchronous), then start again.
func (s *Service) runBinaryRestart(ctx context.Context) processx.Result {
	started := time.Now()
	stop := s.run(ctx, "nginx", "-s", "quit")
	// A failed quit usually means nginx was not running; still wait for any
	// leftover process to exit before trying to bind ports again.
	s.waitForProcessExit(ctx)
	start := s.run(ctx, "nginx")

	output := strings.TrimSpace(strings.Join(filterEmpty(stop.Output, start.Output), "\n"))
	duration := time.Since(started).Milliseconds()
	if start.Err == nil && start.ExitCode == 0 {
		// A failed quit just means nginx was not running before.
		return processx.Result{Stdout: start.Stdout, Output: output, ExitCode: 0, DurationMS: duration}
	}
	if start.Err != nil {
		return processx.Result{Output: output, ExitCode: start.ExitCode, DurationMS: duration, Err: start.Err}
	}
	return processx.Result{Output: output, ExitCode: start.ExitCode, DurationMS: duration, Err: fmt.Errorf("nginx restart failed")}
}

func (s *Service) waitForProcessExit(ctx context.Context) {
	deadline := time.Now().Add(s.commandTimeout())
	for {
		_, running := s.detectProcess(ctx)
		if !running || time.Now().After(deadline) {
			return
		}
		time.Sleep(100 * time.Millisecond)
	}
}

func filterEmpty(values ...string) []string {
	items := make([]string, 0, len(values))
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			items = append(items, v)
		}
	}
	return items
}

func (s *Service) commandTimeout() time.Duration {
	if s.timeout <= 0 {
		return 10 * time.Second
	}
	return s.timeout
}

func (s *Service) detectProcess(ctx context.Context) (*int, bool) {
	result := s.run(ctx, "pgrep", "-x", "nginx")
	if result.Err != nil || result.ExitCode != 0 {
		return nil, false
	}
	for _, line := range strings.Split(result.Stdout, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		pid, err := strconv.Atoi(line)
		if err == nil && pid > 0 {
			return &pid, true
		}
	}
	return nil, false
}

func (s *Service) detectService(ctx context.Context) (string, *string, *string) {
	if s.controlMode == models.NginxControlModeBinary {
		if _, running := s.detectProcess(ctx); running {
			active := "active"
			sub := "running"
			return "binary", &active, &sub
		}
		active := "inactive"
		sub := "dead"
		return "binary", &active, &sub
	}
	activeResult := s.run(ctx, "systemctl", "show", s.serviceName, "--property=ActiveState", "--value")
	if activeResult.Err != nil || activeResult.ExitCode != 0 {
		if _, running := s.detectProcess(ctx); running {
			return "process", nil, nil
		}
		return "unknown", nil, nil
	}

	subResult := s.run(ctx, "systemctl", "show", s.serviceName, "--property=SubState", "--value")
	active := strings.TrimSpace(activeResult.Stdout)
	sub := strings.TrimSpace(subResult.Stdout)
	return "systemd", &active, &sub
}

func parseNginxVersion(output string) *string {
	for _, line := range strings.Split(output, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "nginx version:") {
			version := strings.TrimSpace(strings.TrimPrefix(line, "nginx version:"))
			return &version
		}
	}
	return nil
}

func resultOutput(result processx.Result) string {
	output := strings.TrimSpace(result.Output)
	if result.Err != nil {
		if output != "" {
			output += "\n"
		}
		output += result.Err.Error()
	}
	if len(output) > maxCommandOutputLen {
		output = output[:maxCommandOutputLen] + "...<truncated>"
	}
	return output
}

func validateSiteInput(name, content string) error {
	if err := validateSiteName(name); err != nil {
		return err
	}
	if len(content) > maxSiteContentBytes {
		return apperr.Validationf("nginx site content exceeds %d bytes", maxSiteContentBytes)
	}
	return nil
}

func validateSiteName(name string) error {
	if !dao.ValidSiteName(name) {
		return apperr.Validationf("nginx site name is invalid")
	}
	return nil
}

func mutationResult(summary *apimod.NginxSiteSummary, validation apimod.NginxValidationResult) *apimod.NginxSiteMutationResult {
	return &apimod.NginxSiteMutationResult{
		Name:       summary.Name,
		SizeBytes:  summary.SizeBytes,
		ModifiedAt: summary.ModifiedAt,
		SHA256:     summary.SHA256,
		Validation: validation,
	}
}
