package svc_test

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/KevinZonda/Panel/domain/nginx/svc"
	"github.com/KevinZonda/Panel/lib/apperr"
	"github.com/KevinZonda/Panel/lib/processx"
	"github.com/KevinZonda/Panel/models"
)

type fakeExecutor struct {
	result processx.Result
}

func (f fakeExecutor) Run(_ context.Context, _ string, _ ...string) processx.Result {
	return f.result
}

func TestSiteCRUDWithoutValidation(t *testing.T) {
	root := t.TempDir()
	sitesDir := filepath.Join(root, "sites-enabled")
	backupDir := filepath.Join(root, "backups")
	service := svc.NewService(sitesDir, backupDir, "nginx", "", time.Second, fakeExecutor{})

	created, err := service.CreateSite(context.Background(), "example.com.conf", "server { listen 80; }\n", false)
	if err != nil {
		t.Fatalf("create site: %v", err)
	}
	if created.Name != "example.com.conf" || created.SHA256 == "" {
		t.Fatalf("unexpected create result: %#v", created)
	}

	list, err := service.ListSites()
	if err != nil {
		t.Fatalf("list sites: %v", err)
	}
	if len(list.Items) != 1 {
		t.Fatalf("expected one site, got %d", len(list.Items))
	}

	site, err := service.GetSite("example.com.conf")
	if err != nil {
		t.Fatalf("get site: %v", err)
	}
	if site.Content != "server { listen 80; }\n" {
		t.Fatalf("unexpected content: %q", site.Content)
	}

	if _, err := service.UpdateSite(context.Background(), "example.com.conf", "server { listen 81; }\n", false, "wrong-sha"); !errors.Is(err, apperr.ErrConflict) {
		t.Fatalf("expected sha conflict, got %v", err)
	}
	if _, err := service.UpdateSite(context.Background(), "example.com.conf", "server { listen 81; }\n", false, site.SHA256); err != nil {
		t.Fatalf("update site: %v", err)
	}

	if err := service.DeleteSite("example.com.conf"); err != nil {
		t.Fatalf("delete site: %v", err)
	}
	if _, err := service.GetSite("example.com.conf"); !errors.Is(err, apperr.ErrNotFound) {
		t.Fatalf("expected deleted site to be missing, got %v", err)
	}

	backups, err := os.ReadDir(backupDir)
	if err != nil {
		t.Fatalf("read backup dir: %v", err)
	}
	if len(backups) != 2 {
		t.Fatalf("expected update and delete backups, got %d", len(backups))
	}
}

func TestValidationFailureRollsBack(t *testing.T) {
	root := t.TempDir()
	sitesDir := filepath.Join(root, "sites-enabled")
	backupDir := filepath.Join(root, "backups")
	badExecutor := fakeExecutor{result: processx.Result{ExitCode: 1, Output: "bad config"}}
	service := svc.NewService(sitesDir, backupDir, "nginx", "", time.Second, badExecutor)

	_, err := service.CreateSite(context.Background(), "example.com.conf", "bad nginx config", true)
	if !errors.Is(err, apperr.ErrValidation) {
		t.Fatalf("expected validation error, got %v", err)
	}
	if _, err := os.Stat(filepath.Join(sitesDir, "example.com.conf")); !os.IsNotExist(err) {
		t.Fatalf("expected invalid new site to be rolled back, stat err=%v", err)
	}

	oldContent := "server { listen 80; }\n"
	if _, err := service.CreateSite(context.Background(), "example.com.conf", oldContent, false); err != nil {
		t.Fatalf("create site without validation: %v", err)
	}
	site, err := service.GetSite("example.com.conf")
	if err != nil {
		t.Fatalf("get site: %v", err)
	}
	if _, err := service.UpdateSite(context.Background(), "example.com.conf", "bad nginx config", true, site.SHA256); !errors.Is(err, apperr.ErrValidation) {
		t.Fatalf("expected validation error, got %v", err)
	}

	rolledBack, err := service.GetSite("example.com.conf")
	if err != nil {
		t.Fatalf("get rolled back site: %v", err)
	}
	if rolledBack.Content != oldContent {
		t.Fatalf("expected rollback to old content, got %q", rolledBack.Content)
	}
}

func TestInvalidSiteName(t *testing.T) {
	service := svc.NewService(t.TempDir(), t.TempDir(), "nginx", "", time.Second, fakeExecutor{})
	if _, err := service.CreateSite(context.Background(), "../evil.conf", "", false); !errors.Is(err, apperr.ErrValidation) {
		t.Fatalf("expected validation error, got %v", err)
	}
}

func TestUpdateSitePreservesSymlink(t *testing.T) {
	root := t.TempDir()
	sitesDir := filepath.Join(root, "sites-enabled")
	availableDir := filepath.Join(root, "sites-available")
	if err := os.MkdirAll(sitesDir, 0o755); err != nil {
		t.Fatalf("mkdir sites: %v", err)
	}
	if err := os.MkdirAll(availableDir, 0o755); err != nil {
		t.Fatalf("mkdir available: %v", err)
	}

	target := filepath.Join(availableDir, "example.com.conf")
	if err := os.WriteFile(target, []byte("server { listen 80; }\n"), 0o644); err != nil {
		t.Fatalf("write target: %v", err)
	}
	link := filepath.Join(sitesDir, "example.com.conf")
	if err := os.Symlink(target, link); err != nil {
		t.Fatalf("create symlink: %v", err)
	}

	service := svc.NewService(sitesDir, filepath.Join(root, "backups"), "nginx", "", time.Second, fakeExecutor{})
	site, err := service.GetSite("example.com.conf")
	if err != nil {
		t.Fatalf("get site: %v", err)
	}
	if _, err := service.UpdateSite(context.Background(), "example.com.conf", "server { listen 81; }\n", false, site.SHA256); err != nil {
		t.Fatalf("update site: %v", err)
	}

	info, err := os.Lstat(link)
	if err != nil {
		t.Fatalf("lstat symlink: %v", err)
	}
	if info.Mode()&os.ModeSymlink == 0 {
		t.Fatal("expected sites-enabled entry to remain a symlink")
	}
	content, err := os.ReadFile(target)
	if err != nil {
		t.Fatalf("read target: %v", err)
	}
	if string(content) != "server { listen 81; }\n" {
		t.Fatalf("unexpected target content: %q", string(content))
	}
}

type recordedCall struct {
	name string
	args []string
}

type recordingExecutor struct {
	calls  []recordedCall
	result processx.Result
}

func (r *recordingExecutor) Run(_ context.Context, name string, args ...string) processx.Result {
	r.calls = append(r.calls, recordedCall{name: name, args: args})
	return r.result
}

func TestSystemdModeUsesSystemctl(t *testing.T) {
	executor := &recordingExecutor{}
	service := svc.NewService(t.TempDir(), t.TempDir(), "nginx", models.NginxControlModeSystemd, time.Second, executor)

	for _, action := range []string{"start", "stop", "reload", "restart"} {
		executor.calls = nil
		if _, err := service.Action(context.Background(), action); err != nil {
			t.Fatalf("action %s: %v", action, err)
		}
		if len(executor.calls) == 0 || executor.calls[0].name != "systemctl" {
			t.Fatalf("action %s: expected systemctl call, got %#v", action, executor.calls)
		}
		if len(executor.calls[0].args) != 2 || executor.calls[0].args[0] != action || executor.calls[0].args[1] != "nginx" {
			t.Fatalf("action %s: unexpected systemctl args %#v", action, executor.calls[0].args)
		}
	}
}

func TestBinaryModeUsesNginxBinary(t *testing.T) {
	executor := &recordingExecutor{}
	service := svc.NewService(t.TempDir(), t.TempDir(), "nginx", models.NginxControlModeBinary, time.Second, executor)

	cases := []struct {
		action string
		args   []string
	}{
		{"start", nil},
		{"stop", []string{"-s", "quit"}},
		{"reload", []string{"-s", "reload"}},
	}
	for _, tc := range cases {
		executor.calls = nil
		if _, err := service.Action(context.Background(), tc.action); err != nil {
			t.Fatalf("action %s: %v", tc.action, err)
		}
		if len(executor.calls) == 0 || executor.calls[0].name != "nginx" {
			t.Fatalf("action %s: expected nginx call, got %#v", tc.action, executor.calls)
		}
		if len(executor.calls[0].args) != len(tc.args) {
			t.Fatalf("action %s: unexpected args %#v", tc.action, executor.calls[0].args)
		}
		for i := range tc.args {
			if executor.calls[0].args[i] != tc.args[i] {
				t.Fatalf("action %s: unexpected args %#v", tc.action, executor.calls[0].args)
			}
		}
		// Status detection after the action adds pgrep/nginx -v calls;
		// the first call must be the control command itself.
		if executor.calls[0].name != "nginx" {
			t.Fatalf("action %s: first call must be nginx control command, got %#v", tc.action, executor.calls)
		}
	}
}

func TestBinaryModeRestartIsQuitThenStart(t *testing.T) {
	executor := &recordingExecutor{}
	service := svc.NewService(t.TempDir(), t.TempDir(), "nginx", models.NginxControlModeBinary, time.Second, executor)

	resp, err := service.Action(context.Background(), "restart")
	if err != nil {
		t.Fatalf("restart: %v", err)
	}
	if !resp.OK {
		t.Fatalf("expected restart ok with successful executor, got %#v", resp)
	}

	var nginxCalls []recordedCall
	for _, call := range executor.calls {
		if call.name == "nginx" && len(call.args) == 0 || call.name == "nginx" && len(call.args) == 2 {
			nginxCalls = append(nginxCalls, call)
		}
	}
	if len(nginxCalls) < 2 {
		t.Fatalf("expected quit then start, got %#v", executor.calls)
	}
	if nginxCalls[0].args[0] != "-s" || nginxCalls[0].args[1] != "quit" {
		t.Fatalf("expected first nginx call to be quit, got %#v", nginxCalls[0].args)
	}
	last := nginxCalls[len(nginxCalls)-1]
	if len(last.args) != 0 {
		t.Fatalf("expected final nginx call to be plain start, got %#v", last.args)
	}
}
