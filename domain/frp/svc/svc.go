package svc

import (
	"context"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/KevinZonda/Panel/domain/frp/dao"
	"github.com/KevinZonda/Panel/lib/apperr"
	"github.com/KevinZonda/Panel/lib/processx"
	"github.com/KevinZonda/Panel/models/apimod"
)

const (
	maxConfigContentBytes = 1024 * 1024
	maxCommandOutputLen   = 4096
)

var serviceNamePattern = regexp.MustCompile(`^frp[A-Za-z0-9_.@-]{0,120}\.service$`)

type Service struct {
	frpsDir   string
	frpcDir   string
	backupDir string
	timeout   time.Duration
	executor  processx.Executor
	mu        sync.Mutex
}

func NewService(frpsDir, frpcDir, backupDir string, timeout time.Duration, executor processx.Executor) *Service {
	return &Service{
		frpsDir:   frpsDir,
		frpcDir:   frpcDir,
		backupDir: backupDir,
		timeout:   timeout,
		executor:  executor,
	}
}

func ValidType(configType string) bool {
	return configType == apimod.FrpTypeServer || configType == apimod.FrpTypeClient
}

func (s *Service) rootFor(configType string) (string, error) {
	switch configType {
	case apimod.FrpTypeServer:
		return s.frpsDir, nil
	case apimod.FrpTypeClient:
		return s.frpcDir, nil
	}
	return "", apperr.Validationf("frp config type is invalid")
}

func (s *Service) ListServices(ctx context.Context) (*apimod.FrpServiceList, error) {
	result := s.run(ctx, "systemctl", "list-unit-files", "--type=service", "--no-legend", "--no-pager", "frp*.service")
	if result.Err != nil || result.ExitCode != 0 {
		return nil, apperr.Dependencyf("list frp systemd services failed: %s", resultOutput(result))
	}

	var items []apimod.FrpServiceStatus
	for _, line := range strings.Split(result.Stdout, "\n") {
		fields := strings.Fields(line)
		if len(fields) == 0 || !validServiceName(fields[0]) {
			continue
		}
		unitFileState := ""
		if len(fields) > 1 {
			unitFileState = fields[1]
		}
		items = append(items, s.serviceStatus(ctx, fields[0], unitFileState))
	}
	return &apimod.FrpServiceList{Items: items}, nil
}

func (s *Service) serviceStatus(ctx context.Context, name, fallbackUnitFileState string) apimod.FrpServiceStatus {
	status := apimod.FrpServiceStatus{
		Name:          name,
		Type:          serviceType(name),
		ActiveState:   "unknown",
		SubState:      "unknown",
		UnitFileState: fallbackUnitFileState,
	}

	result := s.run(ctx, "systemctl", "show", name, "--no-pager", "--property=Description,ActiveState,SubState,UnitFileState,MainPID")
	if result.Err == nil && result.ExitCode == 0 {
		props := parseSystemctlShow(result.Stdout)
		if v := props["Description"]; v != "" {
			status.Description = v
		}
		if v := props["ActiveState"]; v != "" {
			status.ActiveState = v
		}
		if v := props["SubState"]; v != "" {
			status.SubState = v
		}
		if v := props["UnitFileState"]; v != "" {
			status.UnitFileState = v
		}
		if v := props["MainPID"]; v != "" {
			if pid, err := strconv.Atoi(v); err == nil && pid > 0 {
				status.MainPID = &pid
			}
		}
	}
	status.Running = status.ActiveState == "active"

	if status.Type == apimod.FrpTypeServer || status.Type == apimod.FrpTypeClient {
		candidate := strings.TrimSuffix(name, ".service") + ".toml"
		if root, err := s.rootFor(status.Type); err == nil && dao.ValidConfigName(candidate) {
			if _, err := os.Stat(filepath.Join(root, candidate)); err == nil {
				status.ConfigType = status.Type
				status.ConfigName = candidate
			}
		}
	}
	return status
}

func serviceType(name string) string {
	if strings.HasPrefix(name, "frps") {
		return apimod.FrpTypeServer
	}
	if strings.HasPrefix(name, "frpc") {
		return apimod.FrpTypeClient
	}
	return "other"
}

func parseSystemctlShow(output string) map[string]string {
	props := map[string]string{}
	for _, line := range strings.Split(output, "\n") {
		key, value, ok := strings.Cut(line, "=")
		if ok {
			props[key] = value
		}
	}
	return props
}

func validServiceName(name string) bool {
	return serviceNamePattern.MatchString(name)
}

func (s *Service) ServiceAction(ctx context.Context, name, action string) (*apimod.FrpServiceActionResponse, error) {
	if !validServiceName(name) {
		return nil, apperr.Validationf("frp service name is invalid")
	}
	switch action {
	case "start", "stop", "restart":
	default:
		return nil, apperr.Validationf("unsupported frp service action")
	}

	s.mu.Lock()
	result := s.run(ctx, "systemctl", action, name)
	s.mu.Unlock()

	return &apimod.FrpServiceActionResponse{
		Service:    name,
		Action:     action,
		OK:         result.Err == nil && result.ExitCode == 0,
		ExitCode:   result.ExitCode,
		Output:     resultOutput(result),
		DurationMS: result.DurationMS,
		FinishedAt: time.Now(),
		Status:     s.serviceStatus(ctx, name, ""),
	}, nil
}

func (s *Service) ListConfigs() (*apimod.FrpConfigList, error) {
	var items []apimod.FrpConfigSummary
	for _, configType := range []string{apimod.FrpTypeServer, apimod.FrpTypeClient} {
		root, _ := s.rootFor(configType)
		configs, err := dao.ListConfigs(root, configType)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return nil, err
		}
		items = append(items, configs...)
	}
	return &apimod.FrpConfigList{Items: items}, nil
}

func (s *Service) GetConfig(configType, name string) (*apimod.FrpConfigFile, error) {
	root, err := s.validatedRoot(configType, name)
	if err != nil {
		return nil, err
	}
	config, err := dao.ReadConfig(root, configType, name)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, apperr.NotFoundf("frp config not found")
		}
		return nil, err
	}
	return config, nil
}

func (s *Service) CreateConfig(ctx context.Context, configType, name, content string, validate bool) (*apimod.FrpConfigMutationResult, error) {
	root, err := s.validatedRoot(configType, name)
	if err != nil {
		return nil, err
	}
	if err := validateConfigContent(content); err != nil {
		return nil, err
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	exists, err := dao.ConfigExists(root, name)
	if err != nil {
		return nil, err
	}
	if exists {
		return nil, apperr.Conflictf("frp config already exists")
	}

	summary, err := dao.WriteConfig(root, configType, name, content)
	if err != nil {
		return nil, apperr.Dependencyf("write frp config failed: %v", err)
	}

	validation := apimod.FrpValidationResult{OK: true, Output: "validation skipped"}
	if validate {
		validation, err = s.validateWrittenConfig(ctx, root, configType, name, false, "")
		if err != nil {
			return nil, err
		}
	}
	return mutationResult(summary, validation), nil
}

func (s *Service) UpdateConfig(ctx context.Context, configType, name, content string, validate bool, expectedSHA256 string) (*apimod.FrpConfigMutationResult, error) {
	root, err := s.validatedRoot(configType, name)
	if err != nil {
		return nil, err
	}
	if err := validateConfigContent(content); err != nil {
		return nil, err
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	oldConfig, err := dao.ReadConfig(root, configType, name)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, apperr.NotFoundf("frp config not found")
		}
		return nil, err
	}
	if expectedSHA256 != "" && expectedSHA256 != oldConfig.SHA256 {
		return nil, apperr.Conflictf("frp config was modified by another writer")
	}
	if _, err := dao.BackupConfig(s.backupDir, configType, name, []byte(oldConfig.Content)); err != nil {
		return nil, apperr.Dependencyf("backup frp config failed: %v", err)
	}

	summary, err := dao.WriteConfig(root, configType, name, content)
	if err != nil {
		return nil, apperr.Dependencyf("write frp config failed: %v", err)
	}

	validation := apimod.FrpValidationResult{OK: true, Output: "validation skipped"}
	if validate {
		validation, err = s.validateWrittenConfig(ctx, root, configType, name, true, oldConfig.Content)
		if err != nil {
			return nil, err
		}
	}
	return mutationResult(summary, validation), nil
}

func (s *Service) DeleteConfig(configType, name string) error {
	root, err := s.validatedRoot(configType, name)
	if err != nil {
		return err
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	oldConfig, err := dao.ReadConfig(root, configType, name)
	if err != nil {
		if os.IsNotExist(err) {
			return apperr.NotFoundf("frp config not found")
		}
		return err
	}
	if _, err := dao.BackupConfig(s.backupDir, configType, name, []byte(oldConfig.Content)); err != nil {
		return apperr.Dependencyf("backup frp config failed: %v", err)
	}
	if err := dao.DeleteConfig(root, name); err != nil {
		if os.IsNotExist(err) {
			return apperr.NotFoundf("frp config not found")
		}
		return err
	}
	return nil
}

func (s *Service) validatedRoot(configType, name string) (string, error) {
	if !ValidType(configType) {
		return "", apperr.Validationf("frp config type is invalid")
	}
	if !dao.ValidConfigName(name) {
		return "", apperr.Validationf("frp config name is invalid")
	}
	return s.rootFor(configType)
}

func validateConfigContent(content string) error {
	if len(content) > maxConfigContentBytes {
		return apperr.Validationf("frp config content exceeds %d bytes", maxConfigContentBytes)
	}
	return nil
}

func (s *Service) validateWrittenConfig(ctx context.Context, root, configType, name string, hadOld bool, oldContent string) (apimod.FrpValidationResult, error) {
	binary := configType
	path := filepath.Join(root, name)
	result := s.run(ctx, binary, "verify", "-c", path)
	validation := apimod.FrpValidationResult{
		OK:     result.Err == nil && result.ExitCode == 0,
		Output: resultOutput(result),
	}
	if validation.OK {
		return validation, nil
	}

	var rollbackErr error
	if hadOld {
		_, rollbackErr = dao.WriteConfig(root, configType, name, oldContent)
	} else {
		rollbackErr = dao.DeleteConfig(root, name)
		if os.IsNotExist(rollbackErr) {
			rollbackErr = nil
		}
	}

	if rollbackErr != nil {
		return validation, apperr.Validationf("frp config validation failed and rollback failed: %v", rollbackErr)
	}
	return validation, apperr.Validationf("frp config validation failed: %s", validation.Output)
}

func mutationResult(summary *apimod.FrpConfigSummary, validation apimod.FrpValidationResult) *apimod.FrpConfigMutationResult {
	return &apimod.FrpConfigMutationResult{
		Type:       summary.Type,
		Name:       summary.Name,
		SizeBytes:  summary.SizeBytes,
		ModifiedAt: summary.ModifiedAt,
		SHA256:     summary.SHA256,
		Validation: validation,
	}
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
