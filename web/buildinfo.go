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

	parts := []string{}
	if info != nil {
		if v := settings["vcs.time"]; v != "" {
			parts = append(parts, v)
		}
		if v := settings["vcs.revision"]; v != "" {
			parts = append(parts, shortSHA(v))
		}
	}
	parts = append(parts, goVersion, compiler)
	return strings.Join(parts, "; ")
}
