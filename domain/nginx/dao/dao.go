package dao

import (
	"os"
	"path/filepath"
	"regexp"
	"time"

	"github.com/KevinZonda/GoX/pkg/cryptox"
	"github.com/KevinZonda/Panel/lib/filex"
	"github.com/KevinZonda/Panel/models/apimod"
)

var siteNamePattern = regexp.MustCompile(`^[A-Za-z0-9._-]{1,128}$`)

func ValidSiteName(name string) bool {
	return siteNamePattern.MatchString(name)
}

func ListSites(dir string) ([]apimod.NginxSiteSummary, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}

	items := make([]apimod.NginxSiteSummary, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() || !ValidSiteName(entry.Name()) {
			continue
		}
		path := filepath.Join(dir, entry.Name())
		info, err := os.Stat(path)
		if err != nil {
			continue
		}
		content, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		items = append(items, apimod.NginxSiteSummary{
			Name:       entry.Name(),
			SizeBytes:  info.Size(),
			ModifiedAt: info.ModTime(),
			SHA256:     cryptox.SHA256(content),
		})
	}
	return items, nil
}

func ReadSite(dir, name string) (*apimod.NginxSite, error) {
	path := filepath.Join(dir, name)
	content, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	info, err := os.Stat(path)
	if err != nil {
		return nil, err
	}
	return &apimod.NginxSite{
		NginxSiteSummary: apimod.NginxSiteSummary{
			Name:       name,
			SizeBytes:  info.Size(),
			ModifiedAt: info.ModTime(),
			SHA256:     cryptox.SHA256(content),
		},
		Content: string(content),
	}, nil
}

func SiteExists(dir, name string) (bool, error) {
	_, err := os.Lstat(filepath.Join(dir, name))
	if err == nil {
		return true, nil
	}
	if os.IsNotExist(err) {
		return false, nil
	}
	return false, err
}

func WriteSite(dir, name, content string) (*apimod.NginxSiteSummary, error) {
	path := filepath.Join(dir, name)
	writePath, err := resolveWritePath(path)
	if err != nil {
		return nil, err
	}
	if err := filex.WriteFileAtomic(writePath, []byte(content), 0o644); err != nil {
		return nil, err
	}
	info, err := os.Stat(writePath)
	if err != nil {
		return nil, err
	}
	return &apimod.NginxSiteSummary{
		Name:       name,
		SizeBytes:  info.Size(),
		ModifiedAt: info.ModTime(),
		SHA256:     cryptox.SHA256Str(content),
	}, nil
}

func resolveWritePath(path string) (string, error) {
	info, err := os.Lstat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return path, nil
		}
		return "", err
	}
	if info.Mode()&os.ModeSymlink == 0 {
		return path, nil
	}

	linkTarget, err := os.Readlink(path)
	if err != nil {
		return "", err
	}
	if !filepath.IsAbs(linkTarget) {
		linkTarget = filepath.Join(filepath.Dir(path), linkTarget)
	}
	return filepath.Clean(linkTarget), nil
}

func DeleteSite(dir, name string) error {
	return os.Remove(filepath.Join(dir, name))
}

func BackupSite(backupDir, name string, content []byte) (string, error) {
	if err := os.MkdirAll(backupDir, 0o700); err != nil {
		return "", err
	}
	filename := name + "." + time.Now().UTC().Format("20060102150405.000000000") + ".bak"
	path := filepath.Join(backupDir, filename)
	return path, filex.WriteFileAtomic(path, content, 0o600)
}
