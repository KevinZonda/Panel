package apimod

import "time"

type LoadAverage struct {
	Load1  float64 `json:"1m"`
	Load5  float64 `json:"5m"`
	Load15 float64 `json:"15m"`
}

type CpuStatus struct {
	UsagePercent     float64     `json:"usage_percent"`
	Cores            int         `json:"cores"`
	PerCorePercent   []float64   `json:"per_core_percent,omitempty"`
	LoadAverage      LoadAverage `json:"load_average"`
	SampleIntervalMS int         `json:"sample_interval_ms"`
	CollectedAt      time.Time   `json:"collected_at"`
}

type SwapStatus struct {
	TotalBytes   uint64  `json:"total_bytes"`
	UsedBytes    uint64  `json:"used_bytes"`
	FreeBytes    uint64  `json:"free_bytes"`
	UsagePercent float64 `json:"usage_percent"`
}

type MemoryStatus struct {
	TotalBytes     uint64     `json:"total_bytes"`
	UsedBytes      uint64     `json:"used_bytes"`
	FreeBytes      uint64     `json:"free_bytes"`
	AvailableBytes uint64     `json:"available_bytes"`
	UsagePercent   float64    `json:"usage_percent"`
	Swap           SwapStatus `json:"swap"`
	CollectedAt    time.Time  `json:"collected_at"`
}

type SystemCpuSummary struct {
	UsagePercent float64     `json:"usage_percent"`
	Cores        int         `json:"cores"`
	LoadAverage  LoadAverage `json:"load_average"`
}

type SystemMemorySummary struct {
	TotalBytes     uint64  `json:"total_bytes"`
	UsedBytes      uint64  `json:"used_bytes"`
	AvailableBytes uint64  `json:"available_bytes"`
	UsagePercent   float64 `json:"usage_percent"`
}

type HostInfo struct {
	Hostname        string    `json:"hostname"`
	OS              string    `json:"os"`
	Platform        string    `json:"platform"`
	PlatformFamily  string    `json:"platform_family"`
	PlatformVersion string    `json:"platform_version"`
	KernelVersion   string    `json:"kernel_version"`
	KernelArch      string    `json:"kernel_arch"`
	BootTime        time.Time `json:"boot_time"`
}

type SystemStatus struct {
	Host          HostInfo            `json:"host"`
	CPU           SystemCpuSummary    `json:"cpu"`
	Memory        SystemMemorySummary `json:"memory"`
	UptimeSeconds uint64              `json:"uptime_seconds"`
	CollectedAt   time.Time           `json:"collected_at"`
}
