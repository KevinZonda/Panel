package filex

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

func TestWriteFileAtomicAndReadFileLimited(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config")
	if err := WriteFileAtomic(path, []byte("first"), 0o640); err != nil {
		t.Fatalf("write atomic: %v", err)
	}
	if err := WriteFileAtomic(path, []byte("second"), 0o640); err != nil {
		t.Fatalf("rewrite atomic: %v", err)
	}
	content, err := ReadFileLimited(path, 16)
	if err != nil {
		t.Fatalf("read limited: %v", err)
	}
	if string(content) != "second" {
		t.Fatalf("content = %q", content)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o640 {
		t.Fatalf("perm = %o", info.Mode().Perm())
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		if strings.HasPrefix(entry.Name(), ".panel-write-") {
			t.Fatalf("temporary file remained: %s", entry.Name())
		}
	}

	large := filepath.Join(dir, "large")
	if err := os.WriteFile(large, []byte("123456789"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := ReadFileLimited(large, 8); err == nil {
		t.Fatal("expected oversized file to be rejected")
	}
}

func TestPruneFilesKeepsNewestMatchingFiles(t *testing.T) {
	dir := t.TempDir()
	for _, name := range []string{
		"site.20240101000000.000000000.bak",
		"site.20240102000000.000000000.bak",
		"site.20240103000000.000000000.bak",
		"site.20240104000000.000000000.bak",
		"other.20240101000000.000000000.bak",
		"site.not-a-backup",
	} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(name), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	if err := PruneFiles(dir, "site.", ".bak", 2); err != nil {
		t.Fatalf("prune files: %v", err)
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, entry := range entries {
		names = append(names, entry.Name())
	}
	sort.Strings(names)
	want := []string{
		"other.20240101000000.000000000.bak",
		"site.20240103000000.000000000.bak",
		"site.20240104000000.000000000.bak",
		"site.not-a-backup",
	}
	if strings.Join(names, ",") != strings.Join(want, ",") {
		t.Fatalf("remaining files = %v, want %v", names, want)
	}
}
