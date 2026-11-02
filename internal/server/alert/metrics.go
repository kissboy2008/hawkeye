package alert

import "hawkeye/internal/models"

// 目标类别：两个入口，互不混装。
const (
	ClassAgent = "agent" // 🖥 机器监控
	ClassProbe = "probe" // 🌐 网页监控
)

// 规则类型。
const (
	KindAvailability = "availability" // 存活型：只判「离线/不可达」，值＝离线分钟数
	KindThreshold    = "threshold"    // 阈值型：metric + op + threshold
)

// MetricAgentOffline 保留这个名字是为了兼容旧数据（历史规则里存的就是这个字符串）。
const MetricAgentOffline = "agent_offline"

// MetricDef 描述一个可配指标的元信息。
//
// 以前引擎里是 `if rule.MetricType == "agent_offline"` 这种硬编码散在三处
// （引擎、探针分支、前端常量表），加一个指标要同时改三处。现在集中在这张表里：
// 引擎按 Class/Availability 决定怎么取值，前端下拉框按 Class 过滤。
type MetricDef struct {
	Key              string  `json:"key"`
	Label            string  `json:"label"`
	Unit             string  `json:"unit"`  // percent | minutes | ms | days | bool
	Class            string  `json:"class"` // agent | probe
	Availability     bool    `json:"availability"`
	JSONKey          string  `json:"-"` // agent 指标 JSON 里的取值字段
	DefaultOperator  string  `json:"default_operator"`
	DefaultThreshold float64 `json:"default_threshold"`
}

var metricRegistry = []MetricDef{
	// ---- 机器监控 ----
	{Key: MetricAgentOffline, Label: "离线时长", Unit: "minutes", Class: ClassAgent,
		Availability: true, DefaultOperator: "gte", DefaultThreshold: 5},
	{Key: "cpu", Label: "CPU 使用率", Unit: "percent", Class: ClassAgent,
		JSONKey: "usage_percent", DefaultOperator: "gt", DefaultThreshold: 90},
	{Key: "memory", Label: "内存使用率", Unit: "percent", Class: ClassAgent,
		JSONKey: "usage_percent", DefaultOperator: "gt", DefaultThreshold: 90},
	{Key: "load1", Label: "负载 1min", Unit: "load", Class: ClassAgent,
		JSONKey: "load1", DefaultOperator: "gt", DefaultThreshold: 8},
	{Key: "load5", Label: "负载 5min", Unit: "load", Class: ClassAgent,
		JSONKey: "load5", DefaultOperator: "gt", DefaultThreshold: 8},
	{Key: "load15", Label: "负载 15min", Unit: "load", Class: ClassAgent,
		JSONKey: "load15", DefaultOperator: "gt", DefaultThreshold: 8},

	// ---- 网页监控 ----
	{Key: "probe_status", Label: "探测状态（1=正常 0=失败）", Unit: "bool", Class: ClassProbe,
		DefaultOperator: "lt", DefaultThreshold: 1},
	{Key: "probe_latency", Label: "响应时间", Unit: "ms", Class: ClassProbe,
		DefaultOperator: "gt", DefaultThreshold: 3000},
	{Key: "cert_expiry", Label: "证书剩余天数", Unit: "days", Class: ClassProbe,
		DefaultOperator: "lt", DefaultThreshold: 14},
}

func MetricDefOf(key string) (MetricDef, bool) {
	for _, m := range metricRegistry {
		if m.Key == key {
			return m, true
		}
	}
	return MetricDef{}, false
}

// MetricsByClass 返回某个入口可用的指标（前端下拉框用）。
func MetricsByClass(class string) []MetricDef {
	out := make([]MetricDef, 0, len(metricRegistry))
	for _, m := range metricRegistry {
		if class == "" || m.Class == class {
			out = append(out, m)
		}
	}
	return out
}

// ClassOfMetric 判断指标属于哪个入口；未知指标按机器处理（保守，避免丢告警）。
func ClassOfMetric(key string) string {
	if def, ok := MetricDefOf(key); ok {
		return def.Class
	}
	return ClassAgent
}

// opSymbol 供消息文案使用。
func opSymbol(op string) string {
	if s, ok := map[string]string{"gt": "＞", "lt": "＜", "gte": "≥", "lte": "≤", "eq": "=", "neq": "≠"}[op]; ok {
		return s
	}
	if op == "" {
		return "＞"
	}
	return op
}

// extractAgentValue 从 agent 指标 JSON 里取值。
// 老实现硬编码了 switch，现在按指标定义里的 JSONKey 取，新指标只加注册表条目。
func extractAgentValue(jsonStr string, def MetricDef) float64 {
	key := def.JSONKey
	if key == "" {
		key = "usage_percent"
	}
	return models.ExtractMetricValue(jsonStr, key)
}
