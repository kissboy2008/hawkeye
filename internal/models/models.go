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
	Tags         string     `json:"tags"`   // comma-separated
	Mode         string     `json:"mode"`   // "push" or "pull"
	Status       string     `json:"status"` // online, offline, unknown
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

// HawkeyeResponse is a flat response for Hawkeye dashboard widget.
type HawkeyeResponse struct {
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
	ID           int64     `json:"id"`
	ProbeID      int64     `json:"probe_id"`
	Timestamp    time.Time `json:"timestamp"`
	StatusCode   int       `json:"status_code"`
	LatencyMs    float64   `json:"latency_ms"`
	Error        string    `json:"error"`
	Success      bool      `json:"success"`
	CertIssuer   string    `json:"cert_issuer,omitempty"`
	CertNotAfter *string   `json:"cert_not_after,omitempty"`
	CertDaysLeft *int      `json:"cert_days_left,omitempty"`
}

// ========== Alert Rules ==========

type AlertRule struct {
	ID              int64   `json:"id"`
	Name            string  `json:"name"`
	Description     string  `json:"description"`
	ScopeType       string  `json:"scope_type"` // "agent" or "probe"
	ScopeID         *int64  `json:"scope_id"`
	MetricType      string  `json:"metric_type"` // cpu, memory, probe_status, probe_latency, cert_expiry
	Operator        string  `json:"operator"`    // gt, lt, gte, lte, eq, neq
	Threshold       float64 `json:"threshold"`
	DurationS       int     `json:"duration_s"`
	Enabled         bool    `json:"enabled"`
	WechatWebhook   string  `json:"wechat_webhook"`
	CooldownS       int     `json:"cooldown_s"`
	RepeatEnabled   bool    `json:"repeat_enabled"`
	RepeatIntervalS int     `json:"repeat_interval_s"`

	// ---- v2 ----
	// TargetClass 决定这条规则属于哪个入口：agent（机器监控）还是 probe（网页监控）。
	// 一条规则只能挂同类目标，避免机器和网站在同一个列表里混着。
	TargetClass string `json:"target_class"`
	// Kind: availability（存活型，只判「离线/不可达」，值=离线分钟数）| threshold（阈值型）
	Kind string `json:"kind"`
	// GroupWaitS 多目标同时告警时的合并窗口（0 = 一个评估周期内合并）
	GroupWaitS int `json:"group_wait_s"`
	// SilencedUntil 整条规则静默到期时间（nil = 不静默）
	SilencedUntil *time.Time `json:"silenced_until"`

	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// AlertRuleTarget 是「一条规则 × 一个目标」的独立配置。
// 每台机器、每个网站一行，阈值/持续/开关/静默都可以单独设；
// 字段为 nil/空 时表示继承规则上的默认值。
type AlertRuleTarget struct {
	ID            int64      `json:"id"`
	RuleID        int64      `json:"rule_id"`
	TargetType    string     `json:"target_type"` // agent | probe
	TargetID      int64      `json:"target_id"`
	Enabled       bool       `json:"enabled"`
	Operator      string     `json:"operator"`  // 空 = 继承
	Threshold     *float64   `json:"threshold"` // nil = 继承
	ForS          *int       `json:"for_s"`     // nil = 继承
	SilencedUntil *time.Time `json:"silenced_until"`
	CreatedAt     time.Time  `json:"created_at"`

	// 只读，接口返回时填充，方便前端一次拿到面板要显示的东西
	TargetName   string     `json:"target_name,omitempty"`
	TargetAddr   string     `json:"target_addr,omitempty"`
	CurrentValue float64    `json:"current_value,omitempty"`
	Status       string     `json:"status,omitempty"`
	FirstTrueAt  *time.Time `json:"first_true_at,omitempty"`
}

// AlertRuleState 是「规则 × 目标」的运行时状态，落库保存。
// 关键点：FirstTrueAt 用墙钟记录「条件首次成立时刻」，
// 服务重启不会清零，持续时长也不再受评估周期精度限制。
type AlertRuleState struct {
	RuleID          int64      `json:"rule_id"`
	TargetType      string     `json:"target_type"`
	TargetID        int64      `json:"target_id"`
	Status          string     `json:"status"` // ok | pending | firing | silenced
	FirstTrueAt     *time.Time `json:"first_true_at"`
	LastEvalAt      *time.Time `json:"last_eval_at"`
	LastValue       float64    `json:"last_value"`
	FiredAt         *time.Time `json:"fired_at"`
	LastNotifyAt    *time.Time `json:"last_notify_at"`
	NotifyCount     int        `json:"notify_count"`
	SuppressedCount int        `json:"suppressed_count"`
}

// AlertOutbox 是通知发送失败后的兜底队列（落库，进程重启不丢）。
type AlertOutbox struct {
	ID          int64     `json:"id"`
	RuleID      int64     `json:"rule_id"`
	Payload     string    `json:"payload"` // 企业微信 text 内容
	Attempts    int       `json:"attempts"`
	NextRetryAt time.Time `json:"next_retry_at"`
	Status      string    `json:"status"` // pending | sent | failed
	LastError   string    `json:"last_error"`
	CreatedAt   time.Time `json:"created_at"`
}

type AlertEvent struct {
	ID         int64      `json:"id"`
	RuleID     int64      `json:"rule_id"`
	AgentID    *int64     `json:"agent_id"`
	ProbeID    *int64     `json:"probe_id"`
	Severity   string     `json:"severity"` // warning, critical
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
	Data       string    `json:"data"` // JSON string
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
