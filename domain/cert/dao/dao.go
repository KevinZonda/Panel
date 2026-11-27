package dao

import (
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"time"

	"github.com/KevinZonda/Panel/lib/filex"
)

const MaxFileBytes = 1024 * 1024

var validName = regexp.MustCompile(`^[A-Za-z0-9._-]{1,128}$`)

// ValidName reports whether a certificate directory name is safe.
// It deliberately matches the nginx site name rules.
func ValidName(name string) bool {
	if !validName.MatchString(name) {
		return false
	}
	if name == "." || name == ".." {
		return false
	}
	return true
}

type FileEntry struct {
	Name       string
	SizeBytes  int64
	ModifiedAt time.Time
}

// ListDomains returns subdirectory names of a cert root, or nil if the
// root does not exist.
func ListDomains(root string) ([]string, error) {
	entries, err := os.ReadDir(root)
	if err != nil {
		return nil, err
	}
	var names []string
	for _, entry := range entries {
		if entry.IsDir() && ValidName(entry.Name()) {
			names = append(names, entry.Name())
		}
	}
	sort.Strings(names)
	return names, nil
}

// ListFiles returns regular files inside a domain directory.
func ListFiles(root, domain string) ([]FileEntry, error) {
	dir := filepath.Join(root, domain)
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	var files []FileEntry
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		info, err := entry.Info()
		if err != nil {
			continue
		}
		files = append(files, FileEntry{
			Name:       entry.Name(),
			SizeBytes:  info.Size(),
			ModifiedAt: info.ModTime(),
		})
	}
	sort.Slice(files, func(i, j int) bool { return files[i].Name < files[j].Name })
	return files, nil
}

func ReadFile(root, domain, name string) ([]byte, error) {
	if !ValidName(domain) || !ValidName(name) {
		return nil, os.ErrInvalid
	}
	return filex.ReadFileLimited(filepath.Join(root, domain, name), MaxFileBytes)
}

// WriteFile atomically writes a file inside the domain directory,
// creating the directory when needed.
func WriteFile(root, domain, name string, content []byte, perm os.FileMode) error {
	if !ValidName(domain) || !ValidName(name) {
		return os.ErrInvalid
	}
	dir := filepath.Join(root, domain)
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return err
	}
	return filex.WriteFileAtomic(filepath.Join(dir, name), content, perm)
}

func DeleteDomain(root, domain string) error {
	if !ValidName(domain) {
		return os.ErrInvalid
	}
	return os.RemoveAll(filepath.Join(root, domain))
}

func DomainExists(root, domain string) bool {
	info, err := os.Stat(filepath.Join(root, domain))
	return err == nil && info.IsDir()
}
