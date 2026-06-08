package models

import (
	"encoding/json"
	"time"
)

// ========== Agent (DB Entity) ==========

type Agent struct {
	ID           int64      `json:"id"`
	Name         string     `json:"name"`
	Address      string     `json:"address"`    // e.g. "http://192.168.1.100:32518"
	ServerURL    string     `json:"server_url"` // push target, e.g. "http://10.0.0.66:80"
	AuthToken    string     `json:"auth_token,omitempty"`
	Tags         string     `json:"tags"`       // comma-separated
	Mode         string     `json:"mode"`       // "push" or "pull"
	Status       string     `json:"status"`     // online, offline, unknown
	AgentVersion string     `json:"agent_version"`
	IntranetURL  string     `json:"intranet_url"` // custom intranet link (left click)
	ExtranetURL  string     `json:"extranet_url"` // custom extranet link (right click)
	SortOrder    int        `json:"sort_order"`
	LastSeen     *time.Time `json:"last_seen"`
	CreatedAt    time.Time  `json:"created_at"`
	UpdatedAt    time.Time  `json:"updated_at"`
}

// ========== Agent Metrics (API Response from Agent) ==========

type AgentMetricsResponse struct {
	Hostname     string        `json:"hostname"`
	Timestamp    time.Time     `json:"timestamp"`
	CPU          CpuMetrics    `json:"cpu"`
	Memory       MemoryMetrics `json:"memory"`
	UptimeS      uint64        `json:"uptime_seconds"`
	AgentVersion string        `json:"agent_version"`
}

type AgentInfoResponse struct {
	Hostname        string `json:"hostname"`
	OS              string `json:"os"`
	Platform        string `json:"platform"`
	PlatformVersion string `json:"platform_version"`
	KernelVersion   string `json:"kernel_version"`
	CPUModel        string `json:"cpu_model"`
	Arch            string `json:"arch"`
	UptimeS         uint64 `json:"uptime_seconds"`
	AgentVer        string `json:"agent_version"`
}

// HomepageResponse is a flat response for homepage.dev Custom API widget.
type HomepageResponse struct {
	Hostname      string  `json:"hostname"`
	CPUModel      string  `json:"cpu_model"`
	CPUPercent    float64 `json:"cpu_percent"`
	CPUCores      int     `json:"cpu_cores"`
	MemoryPercent float64 `json:"memory_percent"`
	MemoryTotalGB float64 `json:"memory_total_gb"`
	MemoryUsedGB  float64 `json:"memory_used_gb"`
	MemoryAvailGB float64 `json:"memory_available_gb"`
	UptimeSeconds uint64  `json:"uptime_seconds"`
	OS            string  `json:"os"`
	OSVersion     string  `json:"os_version"`
	KernelVersion string  `json:"kernel_version"`
	Status        string  `json:"status"`
}

type CpuMetrics struct {
	ModelName     string    `json:"model_name"`
	KernelVersion string    `json:"kernel_version"`
	UsagePercent  float64   `json:"usage_percent"`
	Cores         int       `json:"cores"`
	PerCore       []float64 `json:"per_core"`
	Load1         float64   `json:"load1"`
	Load5         float64   `json:"load5"`
	Load15        float64   `json:"load15"`
}

type MemoryMetrics struct {
	TotalMB      uint64  `json:"total_mb"`
	UsedMB       uint64  `json:"used_mb"`
	AvailableMB  uint64  `json:"available_mb"`
	UsagePercent float64 `json:"usage_percent"`
	SwapTotalMB  uint64  `json:"swap_total_mb"`
	SwapUsedMB   uint64  `json:"swap_used_mb"`
}


type NetMetrics struct {
	Interface string `json:"interface"`
	BytesRecv uint64 `json:"bytes_recv"`
	BytesSent uint64 `json:"bytes_sent"`
}

// ========== Web Probes ==========

type WebProbe struct {
	ID             int64     `json:"id"`
	Name           string    `json:"name"`
	URL            string    `json:"url"`
	Method         string    `json:"method"`
	ExpectedStatus int       `json:"expected_status"`
	TimeoutMs      int       `json:"timeout_ms"`
	IntervalS      int       `json:"interval_s"`
	Enabled        bool      `json:"enabled"`
	SortOrder      int       `json:"sort_order"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}

type ProbeResult struct {
	ID            int64     `json:"id"`
	ProbeID       int64     `json:"probe_id"`
	Timestamp     time.Time `json:"timestamp"`
	StatusCode    int       `json:"status_code"`
	LatencyMs     float64   `json:"latency_ms"`
	Error         string    `json:"error"`
	Success       bool      `json:"success"`
	CertIssuer    string    `json:"cert_issuer,omitempty"`
	CertNotAfter  *string   `json:"cert_not_after,omitempty"`
	CertDaysLeft  *int      `json:"cert_days_left,omitempty"`
}

// ========== Alert Rules ==========

type AlertRule struct {
	ID            int64     `json:"id"`
	Name          string    `json:"name"`
	Description   string    `json:"description"`
	ScopeType     string    `json:"scope_type"`   // "agent" or "probe"
	ScopeID       *int64    `json:"scope_id"`
	MetricType    string    `json:"metric_type"`  // cpu, memory, probe_status, probe_latency, cert_expiry
	Operator      string    `json:"operator"`     // gt, lt, gte, lte, eq, neq
	Threshold     float64   `json:"threshold"`
	DurationS     int       `json:"duration_s"`
	Enabled       bool      `json:"enabled"`
	WechatWebhook string    `json:"wechat_webhook"`
	CooldownS        int       `json:"cooldown_s"`
	RepeatEnabled    bool      `json:"repeat_enabled"`
	RepeatIntervalS  int       `json:"repeat_interval_s"`
	CreatedAt        time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}

type AlertEvent struct {
	ID         int64      `json:"id"`
	RuleID     int64      `json:"rule_id"`
	AgentID    *int64     `json:"agent_id"`
	ProbeID    *int64     `json:"probe_id"`
	Severity   string     `json:"severity"`    // warning, critical
	Message    string     `json:"message"`
	Value      *float64   `json:"value"`
	FiredAt    time.Time  `json:"fired_at"`
	ResolvedAt *time.Time `json:"resolved_at"`
}

// ========== Metric Data Point (stored in DB) ==========

type MetricDataPoint struct {
	ID         int64     `json:"id"`
	AgentID    int64     `json:"agent_id"`
	MetricType string    `json:"metric_type"` // cpu, memory, network
	Timestamp  time.Time `json:"timestamp"`
	Data       string    `json:"data"`        // JSON string
}

// ========== Metric Time Series (API Response) ==========

type MetricsTimeSeries struct {
	MetricType string              `json:"metric_type"`
	DataPoints []MetricPointResult `json:"data_points"`
}

type MetricPointResult struct {
	Timestamp time.Time `json:"timestamp"`
	Value     float64   `json:"value"`
	Labels    string    `json:"labels,omitempty"` // e.g. mount_point or sensor name
}

// ========== Settings ==========

type Setting struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

// ========== Glances API v3 Compatible ==========

type GlancesCPU struct {
	Cpucore   int     `json:"cpucore"`
	Total     float64 `json:"total"`
	User      float64 `json:"user"`
	System    float64 `json:"system"`
	Idle      float64 `json:"idle"`
	Iowait    float64 `json:"iowait"`
}

type GlancesMem struct {
	Total     uint64  `json:"total"`
	Used      uint64  `json:"used"`
	Free      uint64  `json:"free"`
	Available uint64  `json:"available"`
	Percent   float64 `json:"percent"`
}

type GlancesSystem struct {
	Hostname    string `json:"hostname"`
	OSName      string `json:"os_name"`
	OSVersion   string `json:"os_version"`
	LinuxDistro string `json:"linux_distro"`
	HRName      string `json:"hr_name"`
	Platform    string `json:"platform"`
}


// GlancesQuicklook is the /api/3/quicklook response (used by Homepage info metric).
type GlancesQuicklook struct {
	CPU     float64         `json:"cpu"`
	CPUName string          `json:"cpu_name"`
	Mem     float64         `json:"mem"`
	Swap    float64         `json:"swap"`
	PerCPU  []GlancesPerCPU `json:"percpu"`
}

// GlancesPerCPU represents per-CPU core data.
type GlancesPerCPU struct {
	Total float64 `json:"total"`
}

// ExtractMetricValue extracts a float64 value from metric JSON by key.
// Falls back to "usage_percent" if key is empty.
func ExtractMetricValue(jsonStr string, key string) float64 {
	if key == "" {
		key = "usage_percent"
	}
	var data map[string]interface{}
	if err := json.Unmarshal([]byte(jsonStr), &data); err != nil {
		return 0
	}
	if v, ok := data[key].(float64); ok {
		return v
	}
	return 0
}
