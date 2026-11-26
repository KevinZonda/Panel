package svc

import (
	"context"
	"time"

	"github.com/KevinZonda/Panel/models/apimod"
	"github.com/shirou/gopsutil/v3/cpu"
	"github.com/shirou/gopsutil/v3/host"
	"github.com/shirou/gopsutil/v3/load"
	"github.com/shirou/gopsutil/v3/mem"
)

type Service struct{}

func NewService() *Service {
	return &Service{}
}

func (s *Service) CPU(ctx context.Context, interval time.Duration) (*apimod.CpuStatus, error) {
	if interval < 100*time.Millisecond {
		interval = 500 * time.Millisecond
	}
	if interval > 5*time.Second {
		interval = 5 * time.Second
	}

	perCore, err := cpu.PercentWithContext(ctx, interval, true)
	if err != nil {
		return nil, err
	}
	usage := 0.0
	if len(perCore) > 0 {
		for _, v := range perCore {
			usage += v
		}
		usage /= float64(len(perCore))
	}

	cores := len(perCore)
	if cores == 0 {
		cores, err = cpu.CountsWithContext(ctx, true)
		if err != nil {
			return nil, err
		}
	}

	avg, _ := load.AvgWithContext(ctx)
	loadAverage := apimod.LoadAverage{}
	if avg != nil {
		loadAverage = apimod.LoadAverage{Load1: avg.Load1, Load5: avg.Load5, Load15: avg.Load15}
	}

	return &apimod.CpuStatus{
		UsagePercent:     usage,
		Cores:            cores,
		PerCorePercent:   perCore,
		LoadAverage:      loadAverage,
		SampleIntervalMS: int(interval.Milliseconds()),
		CollectedAt:      time.Now(),
	}, nil
}

func (s *Service) Memory(ctx context.Context) (*apimod.MemoryStatus, error) {
	vm, err := mem.VirtualMemoryWithContext(ctx)
	if err != nil {
		return nil, err
	}
	swap, err := mem.SwapMemoryWithContext(ctx)
	if err != nil {
		return nil, err
	}

	return &apimod.MemoryStatus{
		TotalBytes:     vm.Total,
		UsedBytes:      vm.Used,
		FreeBytes:      vm.Free,
		AvailableBytes: vm.Available,
		UsagePercent:   vm.UsedPercent,
		Swap: apimod.SwapStatus{
			TotalBytes:   swap.Total,
			UsedBytes:    swap.Used,
			FreeBytes:    swap.Free,
			UsagePercent: swap.UsedPercent,
		},
		CollectedAt: time.Now(),
	}, nil
}

func (s *Service) Host(ctx context.Context) (*apimod.HostInfo, error) {
	info, err := host.InfoWithContext(ctx)
	if err != nil {
		return nil, err
	}
	return &apimod.HostInfo{
		Hostname:        info.Hostname,
		OS:              info.OS,
		Platform:        info.Platform,
		PlatformFamily:  info.PlatformFamily,
		PlatformVersion: info.PlatformVersion,
		KernelVersion:   info.KernelVersion,
		KernelArch:      info.KernelArch,
		BootTime:        time.Unix(int64(info.BootTime), 0),
	}, nil
}

func (s *Service) Status(ctx context.Context) (*apimod.SystemStatus, error) {
	cpuStatus, err := s.CPU(ctx, 200*time.Millisecond)
	if err != nil {
		return nil, err
	}
	memoryStatus, err := s.Memory(ctx)
	if err != nil {
		return nil, err
	}
	uptime, err := host.UptimeWithContext(ctx)
	if err != nil {
		return nil, err
	}
	hostInfo, err := s.Host(ctx)
	if err != nil {
		return nil, err
	}

	return &apimod.SystemStatus{
		Host: *hostInfo,
		CPU: apimod.SystemCpuSummary{
			UsagePercent: cpuStatus.UsagePercent,
			Cores:        cpuStatus.Cores,
			LoadAverage:  cpuStatus.LoadAverage,
		},
		Memory: apimod.SystemMemorySummary{
			TotalBytes:     memoryStatus.TotalBytes,
			UsedBytes:      memoryStatus.UsedBytes,
			AvailableBytes: memoryStatus.AvailableBytes,
			UsagePercent:   memoryStatus.UsagePercent,
		},
		UptimeSeconds: uptime,
		CollectedAt:   time.Now(),
	}, nil
}
