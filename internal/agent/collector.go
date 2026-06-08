package agent

import (
	"fmt"
	"runtime"
	"time"

	"hawkeye/internal/models"

	"github.com/shirou/gopsutil/v4/cpu"
	"github.com/shirou/gopsutil/v4/host"
	"github.com/shirou/gopsutil/v4/load"
	"github.com/shirou/gopsutil/v4/mem"
)

// Collector gathers system metrics using gopsutil.
type Collector struct {
	version        string
	cpuModel       string // cached CPU model name
	cpuModelCached bool
	kernelVersion  string
}

func NewCollector(version string) *Collector {
	return &Collector{version: version}
}

// CollectMetrics gathers all system metrics in one call.
func (c *Collector) CollectMetrics() (*models.AgentMetricsResponse, error) {
	info, _ := host.Info()
	uptime, _ := host.Uptime()
	resp := &models.AgentMetricsResponse{
		Timestamp:    time.Now().UTC(),
		AgentVersion: c.version,
		UptimeS:      uptime,
	}
	if info != nil {
		resp.Hostname = info.Hostname
		c.kernelVersion = info.KernelVersion
	}

	var err error

	if resp.CPU, err = c.collectCPU(); err != nil {
		return resp, err
	}
	if resp.Memory, err = c.collectMemory(); err != nil {
		return resp, err
	}

	return resp, nil
}

// CollectInfo returns agent host information.
func (c *Collector) CollectInfo() (*models.AgentInfoResponse, error) {
	info, err := host.Info()
	if err != nil {
		return nil, err
	}
	uptime, err := host.Uptime()
	if err != nil {
		return nil, err
	}

	cpuModel := c.getCPUModel()

	return &models.AgentInfoResponse{
		Hostname:        info.Hostname,
		OS:              info.OS,
		Platform:        info.Platform,
		PlatformVersion: info.PlatformVersion,
		KernelVersion:   info.KernelVersion,
		CPUModel:        cpuModel,
		Arch:            runtime.GOARCH,
		UptimeS:         uptime,
		AgentVer:        c.version,
	}, nil
}

func (c *Collector) collectCPU() (models.CpuMetrics, error) {
	cpuModel := c.getCPUModel()

	// Single call — per-core values; total computed as mean to avoid a second blocking call.
	all, err := cpu.Percent(time.Second, true)
	if err != nil {
		return models.CpuMetrics{}, err
	}

	// Compute total from per-core values (same logic as gopsutil aggregate).
	var total float64
	for _, v := range all {
		total += v
	}
	if len(all) > 0 {
		total /= float64(len(all))
	}

	avg, _ := load.Avg()

	return models.CpuMetrics{
		ModelName:     cpuModel,
		KernelVersion: c.kernelVersion,
		UsagePercent:  total,
		Cores:         len(all),
		PerCore:       all,
		Load1:         avg.Load1,
		Load5:         avg.Load5,
		Load15:        avg.Load15,
	}, nil
}

func (c *Collector) collectMemory() (models.MemoryMetrics, error) {
	vm, err := mem.VirtualMemory()
	if err != nil {
		return models.MemoryMetrics{}, err
	}

	const mb = 1024 * 1024

	metrics := models.MemoryMetrics{
		TotalMB:      vm.Total / mb,
		UsedMB:       vm.Used / mb,
		AvailableMB:  vm.Available / mb,
		UsagePercent: vm.UsedPercent,
	}

	// Swap is optional — if it fails, still return VM data
	swap, err := mem.SwapMemory()
	if err == nil {
		metrics.SwapTotalMB = swap.Total / mb
		metrics.SwapUsedMB = swap.Used / mb
	}

	return metrics, nil
}



// getCPUModel returns the CPU model name, cached after first call.
func (c *Collector) getCPUModel() string {
	if c.cpuModelCached {
		return c.cpuModel
	}
	c.cpuModelCached = true

	cpuInfos, err := cpu.Info()
	if err != nil || len(cpuInfos) == 0 {
		return ""
	}
	c.cpuModel = cpuInfos[0].ModelName
	return c.cpuModel
}

// CollectHomepage returns a flat response for homepage.dev Custom API widget.
func (c *Collector) CollectHomepage() (*models.HomepageResponse, error) {
	info, err := host.Info()
	if err != nil {
		return nil, err
	}
	uptime, err := host.Uptime()
	if err != nil {
		return nil, err
	}

	metrics, _ := c.CollectMetrics()

	const mbToGB = 1.0 / 1024.0

	return &models.HomepageResponse{
		Hostname:      info.Hostname,
		CPUModel:      c.getCPUModel(),
		CPUPercent:    metrics.CPU.UsagePercent,
		CPUCores:      metrics.CPU.Cores,
		MemoryPercent: metrics.Memory.UsagePercent,
		MemoryTotalGB: float64(metrics.Memory.TotalMB) * mbToGB,
		MemoryUsedGB:  float64(metrics.Memory.UsedMB) * mbToGB,
		MemoryAvailGB: float64(metrics.Memory.AvailableMB) * mbToGB,
		UptimeSeconds: uptime,
		OS:            info.Platform,
		OSVersion:     info.PlatformVersion,
		KernelVersion: info.KernelVersion,
		Status:        "online",
	}, nil
}

// ========== Glances API v3 Compatible Methods ==========

// CollectGlancesCPU returns CPU data in Glances v3 API format.
func (c *Collector) CollectGlancesCPU() (*models.GlancesCPU, error) {
	cpuMetrics, err := c.collectCPU()
	if err != nil {
		return nil, err
	}

	return &models.GlancesCPU{
		Cpucore: cpuMetrics.Cores,
		Total:   cpuMetrics.UsagePercent,
		Idle:    100 - cpuMetrics.UsagePercent,
	}, nil
}

// CollectGlancesMem returns memory data in Glances v3 API format.
func (c *Collector) CollectGlancesMem() (*models.GlancesMem, error) {
	vm, err := mem.VirtualMemory()
	if err != nil {
		return nil, err
	}

	return &models.GlancesMem{
		Total:     vm.Total,
		Used:      vm.Used,
		Free:      vm.Free,
		Available: vm.Available,
		Percent:   vm.UsedPercent,
	}, nil
}

// CollectGlancesSystem returns system info in Glances v3 API format.
func (c *Collector) CollectGlancesSystem() (*models.GlancesSystem, error) {
	info, err := host.Info()
	if err != nil {
		return nil, err
	}

	platform := "64bit"
	if runtime.GOARCH == "386" || runtime.GOARCH == "arm" {
		platform = "32bit"
	}

	hrName := fmt.Sprintf("%s %s %s", info.Platform, info.PlatformVersion, platform)

	return &models.GlancesSystem{
		Hostname:    info.Hostname,
		OSName:      info.OS,
		OSVersion:   info.KernelVersion,
		LinuxDistro: fmt.Sprintf("%s %s", info.Platform, info.PlatformVersion),
		HRName:      hrName,
		Platform:    platform,
	}, nil
}

// CollectGlancesUptime returns uptime string in Glances v3 format.
func (c *Collector) CollectGlancesUptime() (string, error) {
	uptime, err := host.Uptime()
	if err != nil {
		return "", err
	}

	days := uptime / 86400
	remainder := uptime % 86400
	hours := remainder / 3600
	remainder %= 3600
	minutes := remainder / 60
	seconds := remainder % 60

	return fmt.Sprintf("%d days, %d:%02d:%02d", days, hours, minutes, seconds), nil
}



// CollectGlancesQuicklook returns quick CPU/mem/swap summary in Glances v3 format.
// This is the endpoint called by Homepage's Glances "info" metric.
func (c *Collector) CollectGlancesQuicklook() (*models.GlancesQuicklook, error) {
	cpuMetrics, err := c.collectCPU()
	if err != nil {
		return nil, err
	}

	vm, err := mem.VirtualMemory()
	if err != nil {
		return nil, err
	}

	swap, err := mem.SwapMemory()
	if err != nil {
		return nil, err
	}

	// Build percpu array
	percpu := make([]models.GlancesPerCPU, len(cpuMetrics.PerCore))
	for i, v := range cpuMetrics.PerCore {
		percpu[i] = models.GlancesPerCPU{Total: v}
	}

	var swapPercent float64
	if swap.Total > 0 {
		swapPercent = swap.UsedPercent
	}

	return &models.GlancesQuicklook{
		CPU:     cpuMetrics.UsagePercent,
		CPUName: cpuMetrics.ModelName,
		Mem:     vm.UsedPercent,
		Swap:    swapPercent,
		PerCPU:  percpu,
	}, nil
}
