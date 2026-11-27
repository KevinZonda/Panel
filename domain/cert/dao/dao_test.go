package dao

import (
	"os"
	"path/filepath"
	"testing"
)

func TestReadFileRejectsUnexpectedlyLargeFile(t *testing.T) {
	root := t.TempDir()
	domainDir := filepath.Join(root, "example.com")
	if err := os.MkdirAll(domainDir, 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(domainDir, "pub"), make([]byte, MaxFileBytes+1), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := ReadFile(root, "example.com", "pub"); err == nil {
		t.Fatal("expected oversized cert file to be rejected")
	}
}
