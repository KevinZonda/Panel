package web

import (
	"runtime"
	"runtime/debug"
	"strings"
)

func buildInfoSummary() string {
	info, ok := debug.ReadBuildInfo()
	if !ok {
		info = nil
	}
	return formatBuildInfo(info)
}

func formatBuildInfo(info *debug.BuildInfo) string {
	settings := map[string]string{}
	if info != nil {
		for _, setting := range info.Settings {
			settings[setting.Key] = setting.Value
		}
	}

	goVersion := runtime.Version()
	if info != nil && info.GoVersion != "" {
		goVersion = info.GoVersion
	}
	compiler := runtime.Compiler
	if v := settings["-compiler"]; v != "" {
		compiler = v
	}
	goos := runtime.GOOS
	if v := settings["GOOS"]; v != "" {
		goos = v
	}
	goarch := runtime.GOARCH
	if v := settings["GOARCH"]; v != "" {
		goarch = v
	}

	target := goos + "/" + goarch
	for _, key := range []string{"GO386", "GOAMD64", "GOARM", "GOARM64", "GOMIPS", "GOMIPS64", "GOPPC64", "GORISCV64"} {
		if v := settings[key]; v != "" {
			target += "/" + v
			break
		}
	}

	parts := []string{
		"go=" + goVersion,
		"builder=" + compiler + "/" + goVersion,
		"target=" + target,
	}
	if v := settings["CGO_ENABLED"]; v != "" {
		parts = append(parts, "cgo="+v)
	}
	if v := settings["-buildmode"]; v != "" {
		parts = append(parts, "mode="+v)
	}
	if info != nil {
		if v := info.Main.Version; v != "" && v != "(devel)" {
			parts = append(parts, "mod="+v)
		}
		if v := settings["vcs.revision"]; v != "" {
			parts = append(parts, "rev="+shortSHA(v))
		}
		if v := settings["vcs.time"]; v != "" {
			parts = append(parts, "vcs-time="+v)
		}
		if settings["vcs.modified"] == "true" {
			parts = append(parts, "dirty")
		}
	}
	return strings.Join(parts, "; ")
}
