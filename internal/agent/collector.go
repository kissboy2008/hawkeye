package agent

import (
	"errors"
	"log"
	"runtime"
	"sync"
	"time"

	"hawkeye/internal/models"

	"github.com/shirou/gopsutil/v4/cpu"
	"github.com/shirou/gopsutil/v4/host"
	"github.com/shirou/gopsutil/v4/load"
	"github.com/shirou/gopsutil/v4/mem"
)

// Collector gathers system metrics using gopsutil.
//
// 并发说明：同一个 Collector 会被两条路径同时使用——HTTP /metrics 处理器（前端点开详情时）
// 和 reporter 的定时上报 goroutine。cpuModel / kernelVersion 是写在读之前，但没有任何同步，
// 属于真实的数据竞争（-race 能报出来），因此这里用 sync.Once + RWMutex 保护。
type Collector struct {
	version  string
	cpuModel sync.Once
	cpuName  string

	mu             sync.RWMutex
	kernelVersion  string
	sampleInterval time.Duration // cpu.Percent 的采样窗口
	cpuModelValue  string
	lastPerCore    []float64 // 后台采样器写入的最近一次 per-core 占用率
	samplerOnce    sync.Once // 后台采样器只启动一次
}

func NewCollector(version string) *Collector {
	return &Collector{version: version, sampleInterval: time.Second}
}

// SetSampleInterval 调整 CPU 采样窗口（cpu.Percent 会阻塞这么久）。
// 默认 1s：与上报间隔相比开销可接受；容器里想更轻可以调到 200ms。
func (c *Collector) SetSampleInterval(d time.Duration) {
	if d <= 0 {
		return
	}
	c.mu.Lock()
	c.sampleInterval = d
	c.mu.Unlock()
}

func (c *Collector) getSampleInterval() time.Duration {
	c.mu.RLock()
	defer c.mu.RUnlock()
	if c.sampleInterval <= 0 {
		return time.Second
	}
	return c.sampleInterval
}

func (c *Collector) setKernelVersion(v string) {
	c.mu.Lock()
	c.kernelVersion = v
	c.mu.Unlock()
}

func (c *Collector) getKernelVersion() string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.kernelVersion
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
		c.setKernelVersion(info.KernelVersion)
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

// cpuPercent 返回最近一次 per-core CPU 占用率。
//
// 旧实现每次 CollectMetrics 都调用 cpu.Percent(1s, true)：HTTP /metrics 处理器和
// reporter 都会各自阻塞 1 秒，前端点开详情时会明显卡顿。现在改成后台常驻采样器
// （按 sampleInterval 连续采样，样本永远新鲜），CollectMetrics 只读缓存值，零阻塞。
// 只有进程刚启动、采样器还没产出第一个样本时，才会同步阻塞一次。
func (c *Collector) cpuPercent() []float64 {
	c.samplerOnce.Do(func() {
		go func() {
			for {
				vals, err := cpu.Percent(c.getSampleInterval(), true) // 阻塞一个窗口后返回这段时间的占用率
				if err != nil || len(vals) == 0 {
					time.Sleep(time.Second)
					continue
				}
				c.mu.Lock()
				c.lastPerCore = vals
				c.mu.Unlock()
			}
		}()
	})

	c.mu.RLock()
	vals := c.lastPerCore
	c.mu.RUnlock()
	if len(vals) > 0 {
		return vals
	}

	// 冷启动：同步采一次（仅第一次），随后由后台采样器接管
	vals, err := cpu.Percent(c.getSampleInterval(), true)
	if err != nil || len(vals) == 0 {
		return nil
	}
	c.mu.Lock()
	c.lastPerCore = vals
	c.mu.Unlock()
	return vals
}

func (c *Collector) collectCPU() (models.CpuMetrics, error) {
	cpuModel := c.getCPUModel()

	all := c.cpuPercent()
	if all == nil {
		return models.CpuMetrics{}, errors.New("cpu.Percent failed")
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
		KernelVersion: c.getKernelVersion(),
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
// sync.Once 保证并发调用只探测一次，也不会出现"读到 true 但 name 还是空"的窗口。
func (c *Collector) getCPUModel() string {
	c.cpuModel.Do(func() {
		cpuInfos, err := cpu.Info()
		if err != nil || len(cpuInfos) == 0 {
			return
		}
		c.mu.Lock()
		c.cpuModelValue = cpuInfos[0].ModelName
		c.mu.Unlock()
	})
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.cpuModelValue
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
