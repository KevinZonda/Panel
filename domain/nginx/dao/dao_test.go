package dao

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestBackupSitePrunesToRetention(t *testing.T) {
	backupDir := t.TempDir()
	for i := 0; i < backupRetention+5; i++ {
		if _, err := BackupSite(backupDir, "example.com.conf", []byte{'a' + byte(i%26)}); err != nil {
			t.Fatalf("backup site %d: %v", i, err)
		}
		time.Sleep(time.Millisecond)
	}
	entries, err := os.ReadDir(backupDir)
	if err != nil {
		t.Fatal(err)
	}
	count := 0
	for _, entry := range entries {
		if strings.HasPrefix(entry.Name(), "example.com.conf.") && strings.HasSuffix(entry.Name(), ".bak") {
			count++
		}
	}
	if count != backupRetention {
		t.Fatalf("backup count = %d, want %d", count, backupRetention)
	}
}

func TestReadSiteRejectsUnexpectedlyLargeFile(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "example.com.conf"), make([]byte, maxSiteFileBytes+1), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := ReadSite(dir, "example.com.conf"); err == nil {
		t.Fatal("expected oversized site file to be rejected")
	}
}
