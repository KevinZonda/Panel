package dao

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/KevinZonda/GoX/pkg/cryptox"
	"github.com/KevinZonda/Panel/lib/filex"
	"github.com/KevinZonda/Panel/models/apimod"
)

const (
	maxConfigFileBytes = 1024 * 1024
	backupRetention    = 50
)

var configNamePattern = regexp.MustCompile(`^[A-Za-z0-9._-]{1,128}$`)

func ValidConfigName(name string) bool {
	if !configNamePattern.MatchString(name) || !strings.HasSuffix(name, ".toml") {
		return false
	}
	return strings.TrimSuffix(name, ".toml") != ""
}

func ServiceNameForConfig(name string) string {
	return strings.TrimSuffix(name, ".toml") + ".service"
}

func ListConfigs(dir, configType string) ([]apimod.FrpConfigSummary, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}

	items := make([]apimod.FrpConfigSummary, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() || !ValidConfigName(entry.Name()) {
			continue
		}
		path := filepath.Join(dir, entry.Name())
		info, err := os.Stat(path)
		if err != nil {
			continue
		}
		sha256, err := filex.SHA256File(path)
		if err != nil {
			continue
		}
		items = append(items, summary(configType, entry.Name(), info, sha256))
	}
	return items, nil
}

func ReadConfig(dir, configType, name string) (*apimod.FrpConfigFile, error) {
	path := filepath.Join(dir, name)
	content, err := filex.ReadFileLimited(path, maxConfigFileBytes)
	if err != nil {
		return nil, err
	}
	info, err := os.Stat(path)
	if err != nil {
		return nil, err
	}
	return &apimod.FrpConfigFile{
		FrpConfigSummary: summary(configType, name, info, cryptox.SHA256(content)),
		Content:          string(content),
	}, nil
}

func ConfigExists(dir, name string) (bool, error) {
	_, err := os.Lstat(filepath.Join(dir, name))
	if err == nil {
		return true, nil
	}
	if os.IsNotExist(err) {
		return false, nil
	}
	return false, err
}

func WriteConfig(dir, configType, name, content string) (*apimod.FrpConfigSummary, error) {
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
	item := summary(configType, name, info, cryptox.SHA256Str(content))
	return &item, nil
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

func DeleteConfig(dir, name string) error {
	return os.Remove(filepath.Join(dir, name))
}

func BackupConfig(backupDir, configType, name string, content []byte) (string, error) {
	dir := filepath.Join(backupDir, configType)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", err
	}
	filename := name + "." + time.Now().UTC().Format("20060102150405.000000000") + ".bak"
	path := filepath.Join(dir, filename)
	if err := filex.WriteFileAtomic(path, content, 0o600); err != nil {
		return "", err
	}
	if err := filex.PruneFiles(dir, name+".", ".bak", backupRetention); err != nil {
		return "", err
	}
	return path, nil
}

func summary(configType, name string, info os.FileInfo, sha256 string) apimod.FrpConfigSummary {
	return apimod.FrpConfigSummary{
		Type:        configType,
		Name:        name,
		ServiceName: ServiceNameForConfig(name),
		SizeBytes:   info.Size(),
		ModifiedAt:  info.ModTime(),
		SHA256:      sha256,
	}
}
