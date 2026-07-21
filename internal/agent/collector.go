package agent

import (
	"log"
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
	info, err := host.Info()
	if err != nil {
		log.Printf("[collector] host.Info failed: %v", err)
	}
	uptime, err := host.Uptime()
	if err != nil {
		log.Printf("[collector] host.Uptime failed: %v", err)
	}
	resp := &models.AgentMetricsResponse{
		Timestamp:    time.Now().UTC(),
		AgentVersion: c.version,
		UptimeS:      uptime,
	}
	if info != nil {
		resp.Hostname = info.Hostname
		c.kernelVersion = info.KernelVersion
	}


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

// CollectHawkeye returns a flat response for Hawkeye dashboard widget.
func (c *Collector) CollectHawkeye() (*models.HawkeyeResponse, error) {
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

	return &models.HawkeyeResponse{
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
