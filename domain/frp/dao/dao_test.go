package dao

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/KevinZonda/Panel/models/apimod"
)

func TestBackupConfigPrunesToRetention(t *testing.T) {
	backupDir := t.TempDir()
	for i := 0; i < backupRetention+5; i++ {
		if _, err := BackupConfig(backupDir, apimod.FrpTypeServer, "frps-academic.toml", []byte{'a' + byte(i%26)}); err != nil {
			t.Fatalf("backup config %d: %v", i, err)
		}
		time.Sleep(time.Millisecond)
	}
	entries, err := os.ReadDir(filepath.Join(backupDir, apimod.FrpTypeServer))
	if err != nil {
		t.Fatal(err)
	}
	count := 0
	for _, entry := range entries {
		if strings.HasPrefix(entry.Name(), "frps-academic.toml.") && strings.HasSuffix(entry.Name(), ".bak") {
			count++
		}
	}
	if count != backupRetention {
		t.Fatalf("backup count = %d, want %d", count, backupRetention)
	}
}

func TestReadConfigRejectsUnexpectedlyLargeFile(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "frps-academic.toml"), make([]byte, maxConfigFileBytes+1), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := ReadConfig(dir, apimod.FrpTypeServer, "frps-academic.toml"); err == nil {
		t.Fatal("expected oversized config file to be rejected")
	}
}
