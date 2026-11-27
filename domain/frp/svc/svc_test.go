package svc_test

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/KevinZonda/Panel/domain/frp/svc"
	"github.com/KevinZonda/Panel/lib/apperr"
	"github.com/KevinZonda/Panel/lib/processx"
	"github.com/KevinZonda/Panel/models/apimod"
)

type recordedCall struct {
	name string
	args []string
}

type fakeExecutor struct {
	calls   []recordedCall
	handler func(name string, args ...string) processx.Result
}

func (f *fakeExecutor) Run(_ context.Context, name string, args ...string) processx.Result {
	f.calls = append(f.calls, recordedCall{name: name, args: append([]string(nil), args...)})
	if f.handler != nil {
		return f.handler(name, args...)
	}
	return processx.Result{}
}

func setup(t *testing.T, executor processx.Executor) (*svc.Service, string, string, string) {
	t.Helper()
	root := t.TempDir()
	frpsDir := filepath.Join(root, "frps")
	frpcDir := filepath.Join(root, "frp")
	backupDir := filepath.Join(root, "backups")
	return svc.NewService(frpsDir, frpcDir, backupDir, time.Second, executor), frpsDir, frpcDir, backupDir
}

func TestConfigCRUDWithoutValidation(t *testing.T) {
	service, _, _, backupDir := setup(t, &fakeExecutor{})

	created, err := service.CreateConfig(context.Background(), apimod.FrpTypeServer, "frps-academic.toml", "bindPort = 7000\n", false)
	if err != nil {
		t.Fatalf("create config: %v", err)
	}
	if created.Name != "frps-academic.toml" || created.SHA256 == "" {
		t.Fatalf("unexpected create result: %#v", created)
	}
	if _, err := service.CreateConfig(context.Background(), apimod.FrpTypeServer, "frps-academic.toml", "bindPort = 7001\n", false); !errors.Is(err, apperr.ErrConflict) {
		t.Fatalf("expected conflict, got %v", err)
	}

	list, err := service.ListConfigs()
	if err != nil {
		t.Fatalf("list configs: %v", err)
	}
	if len(list.Items) != 1 {
		t.Fatalf("expected one config, got %d", len(list.Items))
	}
	if list.Items[0].Type != apimod.FrpTypeServer || list.Items[0].ServiceName != "frps-academic.service" {
		t.Fatalf("unexpected list item: %#v", list.Items[0])
	}

	config, err := service.GetConfig(apimod.FrpTypeServer, "frps-academic.toml")
	if err != nil {
		t.Fatalf("get config: %v", err)
	}
	if config.Content != "bindPort = 7000\n" {
		t.Fatalf("unexpected content: %q", config.Content)
	}
	if _, err := service.UpdateConfig(context.Background(), apimod.FrpTypeServer, "frps-academic.toml", "bindPort = 7001\n", false, "wrong-sha"); !errors.Is(err, apperr.ErrConflict) {
		t.Fatalf("expected sha conflict, got %v", err)
	}
	if _, err := service.UpdateConfig(context.Background(), apimod.FrpTypeServer, "frps-academic.toml", "bindPort = 7001\n", false, config.SHA256); err != nil {
		t.Fatalf("update config: %v", err)
	}

	if err := service.DeleteConfig(apimod.FrpTypeServer, "frps-academic.toml"); err != nil {
		t.Fatalf("delete config: %v", err)
	}
	if _, err := service.GetConfig(apimod.FrpTypeServer, "frps-academic.toml"); !errors.Is(err, apperr.ErrNotFound) {
		t.Fatalf("expected deleted config to be missing, got %v", err)
	}

	backups, err := os.ReadDir(filepath.Join(backupDir, apimod.FrpTypeServer))
	if err != nil {
		t.Fatalf("read backup dir: %v", err)
	}
	if len(backups) != 2 {
		t.Fatalf("expected update and delete backups, got %d", len(backups))
	}
}

func TestValidationFailureRollsBack(t *testing.T) {
	executor := &fakeExecutor{handler: func(name string, args ...string) processx.Result {
		return processx.Result{ExitCode: 1, Output: "bad frp config"}
	}}
	service, _, _, _ := setup(t, executor)

	_, err := service.CreateConfig(context.Background(), apimod.FrpTypeServer, "frps-academic.toml", "bad toml", true)
	if !errors.Is(err, apperr.ErrValidation) {
		t.Fatalf("expected validation error, got %v", err)
	}
	if _, err := service.GetConfig(apimod.FrpTypeServer, "frps-academic.toml"); !errors.Is(err, apperr.ErrNotFound) {
		t.Fatalf("expected invalid new config to be rolled back, got %v", err)
	}

	oldContent := "bindPort = 7000\n"
	if _, err := service.CreateConfig(context.Background(), apimod.FrpTypeServer, "frps-academic.toml", oldContent, false); err != nil {
		t.Fatalf("create config without validation: %v", err)
	}
	config, err := service.GetConfig(apimod.FrpTypeServer, "frps-academic.toml")
	if err != nil {
		t.Fatalf("get config: %v", err)
	}
	if _, err := service.UpdateConfig(context.Background(), apimod.FrpTypeServer, "frps-academic.toml", "bad toml", true, config.SHA256); !errors.Is(err, apperr.ErrValidation) {
		t.Fatalf("expected validation error, got %v", err)
	}

	rolledBack, err := service.GetConfig(apimod.FrpTypeServer, "frps-academic.toml")
	if err != nil {
		t.Fatalf("get rolled back config: %v", err)
	}
	if rolledBack.Content != oldContent {
		t.Fatalf("expected rollback to old content, got %q", rolledBack.Content)
	}
}

func TestVerifyUsesTypeBinary(t *testing.T) {
	executor := &fakeExecutor{}
	service, frpsDir, frpcDir, _ := setup(t, executor)

	if _, err := service.CreateConfig(context.Background(), apimod.FrpTypeServer, "frps-academic.toml", "bindPort = 7000\n", true); err != nil {
		t.Fatalf("create frps config: %v", err)
	}
	if len(executor.calls) != 1 || executor.calls[0].name != "frps" {
		t.Fatalf("expected frps verify call, got %#v", executor.calls)
	}
	wantPath := filepath.Join(frpsDir, "frps-academic.toml")
	if strings.Join(executor.calls[0].args, " ") != "verify -c "+wantPath {
		t.Fatalf("unexpected frps verify args: %#v", executor.calls[0].args)
	}

	executor.calls = nil
	if _, err := service.CreateConfig(context.Background(), apimod.FrpTypeClient, "frpc.toml", "serverAddr = \"example.com\"\n", true); err != nil {
		t.Fatalf("create frpc config: %v", err)
	}
	if len(executor.calls) != 1 || executor.calls[0].name != "frpc" {
		t.Fatalf("expected frpc verify call, got %#v", executor.calls)
	}
	wantPath = filepath.Join(frpcDir, "frpc.toml")
	if strings.Join(executor.calls[0].args, " ") != "verify -c "+wantPath {
		t.Fatalf("unexpected frpc verify args: %#v", executor.calls[0].args)
	}
}

func TestListServicesParsesSystemdUnits(t *testing.T) {
	executor := &fakeExecutor{handler: func(name string, args ...string) processx.Result {
		if name != "systemctl" {
			return processx.Result{}
		}
		if len(args) > 0 && args[0] == "list-unit-files" {
			return processx.Result{Stdout: "frps.service enabled\nfrps-academic.service disabled\nfrpc.service enabled\n"}
		}
		if len(args) > 1 && args[0] == "show" {
			switch args[1] {
			case "frps-academic.service":
				return processx.Result{Stdout: "Description=academic frp server\nActiveState=active\nSubState=running\nUnitFileState=disabled\nMainPID=1234\n"}
			default:
				return processx.Result{Stdout: "Description=frp service\nActiveState=inactive\nSubState=dead\nUnitFileState=enabled\nMainPID=0\n"}
			}
		}
		return processx.Result{}
	}}
	service, frpsDir, _, _ := setup(t, executor)
	if err := os.MkdirAll(frpsDir, 0o755); err != nil {
		t.Fatalf("mkdir frps dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(frpsDir, "frps-academic.toml"), []byte("bindPort = 7000\n"), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	list, err := service.ListServices(context.Background())
	if err != nil {
		t.Fatalf("list services: %v", err)
	}
	if len(list.Items) != 3 {
		t.Fatalf("expected 3 services, got %d", len(list.Items))
	}
	academic := list.Items[1]
	if academic.Name != "frps-academic.service" || academic.Type != apimod.FrpTypeServer {
		t.Fatalf("unexpected academic service: %#v", academic)
	}
	if !academic.Running || academic.MainPID == nil || *academic.MainPID != 1234 {
		t.Fatalf("unexpected running state: %#v", academic)
	}
	if academic.ConfigType != apimod.FrpTypeServer || academic.ConfigName != "frps-academic.toml" {
		t.Fatalf("expected config association, got %#v", academic)
	}
	if list.Items[2].Type != apimod.FrpTypeClient {
		t.Fatalf("expected frpc type, got %#v", list.Items[2])
	}

	serviceStatus, err := service.GetService(context.Background(), "frps-academic.service")
	if err != nil {
		t.Fatalf("get service: %v", err)
	}
	if serviceStatus.Name != "frps-academic.service" || !serviceStatus.Running {
		t.Fatalf("unexpected service status: %#v", serviceStatus)
	}
	if _, err := service.GetService(context.Background(), "frps-missing.service"); !errors.Is(err, apperr.ErrNotFound) {
		t.Fatalf("expected missing service error, got %v", err)
	}
	if _, err := service.GetService(context.Background(), "not-frp.service"); !errors.Is(err, apperr.ErrValidation) {
		t.Fatalf("expected invalid service error, got %v", err)
	}
}

func TestServiceActionUsesSystemctl(t *testing.T) {
	executor := &fakeExecutor{handler: func(name string, args ...string) processx.Result {
		if name == "systemctl" && len(args) > 0 && args[0] == "show" {
			return processx.Result{Stdout: "Description=frp service\nActiveState=active\nSubState=running\nUnitFileState=enabled\nMainPID=1234\n"}
		}
		return processx.Result{}
	}}
	service, _, _, _ := setup(t, executor)

	resp, err := service.ServiceAction(context.Background(), "frps-academic.service", "restart")
	if err != nil {
		t.Fatalf("service action: %v", err)
	}
	if !resp.OK || !resp.Status.Running {
		t.Fatalf("unexpected action response: %#v", resp)
	}
	if len(executor.calls) == 0 || executor.calls[0].name != "systemctl" {
		t.Fatalf("expected systemctl call, got %#v", executor.calls)
	}
	if strings.Join(executor.calls[0].args, " ") != "restart frps-academic.service" {
		t.Fatalf("unexpected systemctl args: %#v", executor.calls[0].args)
	}
	if _, err := service.ServiceAction(context.Background(), "nginx.service", "restart"); !errors.Is(err, apperr.ErrValidation) {
		t.Fatalf("expected non-frp service rejection, got %v", err)
	}
	if _, err := service.ServiceAction(context.Background(), "frps.service", "reload"); !errors.Is(err, apperr.ErrValidation) {
		t.Fatalf("expected unsupported action rejection, got %v", err)
	}
}

func TestInvalidConfigInput(t *testing.T) {
	service, _, _, _ := setup(t, &fakeExecutor{})
	if _, err := service.CreateConfig(context.Background(), apimod.FrpTypeServer, "../evil.toml", "", false); !errors.Is(err, apperr.ErrValidation) {
		t.Fatalf("expected validation error for path traversal, got %v", err)
	}
	if _, err := service.CreateConfig(context.Background(), apimod.FrpTypeServer, "frps.ini", "", false); !errors.Is(err, apperr.ErrValidation) {
		t.Fatalf("expected validation error for non-toml file, got %v", err)
	}
	if _, err := service.CreateConfig(context.Background(), "other", "frps.toml", "", false); !errors.Is(err, apperr.ErrValidation) {
		t.Fatalf("expected validation error for unknown type, got %v", err)
	}
}
