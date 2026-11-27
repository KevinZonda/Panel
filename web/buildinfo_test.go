package web

import (
	"runtime/debug"
	"strings"
	"testing"
)

func TestFormatBuildInfoIncludesTimeRevisionToolchainAndCompiler(t *testing.T) {
	info := &debug.BuildInfo{
		GoVersion: "go1.26.10",
		Main:      debug.Module{Version: "v1.2.3"},
		Settings: []debug.BuildSetting{
			{Key: "-compiler", Value: "gc"},
			{Key: "-buildmode", Value: "exe"},
			{Key: "CGO_ENABLED", Value: "1"},
			{Key: "GOOS", Value: "linux"},
			{Key: "GOARCH", Value: "amd64"},
			{Key: "GOAMD64", Value: "v3"},
			{Key: "vcs.revision", Value: "0123456789abcdef0123456789abcdef01234567"},
			{Key: "vcs.time", Value: "2026-11-26T06:35:00Z"},
			{Key: "vcs.modified", Value: "true"},
		},
	}

	got := formatBuildInfo(info)
	want := "2026-11-26T06:35:00Z; 0123456789ab; go1.26.10; gc"
	if got != want {
		t.Fatalf("unexpected build info:\nwant: %s\ngot:  %s", want, got)
	}
}

func TestFormatBuildInfoFallsBackWithoutBuildInfo(t *testing.T) {
	got := formatBuildInfo(nil)
	for _, part := range []string{"go", "gc"} {
		if !strings.Contains(got, part) {
			t.Fatalf("fallback build info missing %q: %q", part, got)
		}
	}
}
