export interface Agent {
  id: number
  name: string
  address: string
  server_url?: string
  auth_token?: string
  tags: string
  mode: 'push' | 'pull'
  agent_version: string
  intranet_url: string
  extranet_url: string
  status: 'online' | 'offline' | 'unknown'
  sort_order: number
  last_seen: string | null
  created_at: string
  updated_at: string
}

export interface CpuMetrics {
  usage_percent: number
  cores: number
  per_core: number[]
}

export interface MemoryMetrics {
  total_mb: number
  used_mb: number
  available_mb: number
  usage_percent: number
  swap_total_mb: number
  swap_used_mb: number
}

export interface AgentMetrics {
  hostname: string
  timestamp: string
  cpu: CpuMetrics
  memory: MemoryMetrics
}

export interface WebProbe {
  id: number
  name: string
  url: string
  method: string
  expected_status: number
  timeout_ms: number
  interval_s: number
  enabled: boolean
  sort_order: number
  created_at: string
  updated_at: string
}

export interface ProbeResult {
  id: number
  probe_id: number
  timestamp: string
  status_code: number
  latency_ms: number
  error: string
  success: boolean
  cert_issuer?: string
  cert_not_after?: string
  cert_days_left?: number
}

export interface ProbeStats {
  probe_id: number
  total_checks: number
  success_count: number
  avg_latency_ms: number
  up_percent: number
}

export interface CertInfo {
  issuer: string
  not_after: string
  days_left: number
}

export interface UptimeBar {
  label: string
  up: boolean | null  // null = no data, true = up, false = down
}

export interface AlertRule {
  id: number
  name: string
  description: string
  scope_type: 'agent' | 'probe'
  scope_id: number | null
  metric_type: string
  operator: string
  threshold: number
  duration_s: number
  enabled: boolean
  wechat_webhook: string
  cooldown_s: number
  repeat_enabled: boolean
  repeat_interval_s: number
  created_at: string
  updated_at: string
}

export interface AlertEvent {
  id: number
  rule_id: number
  agent_id: number | null
  probe_id: number | null
  severity: 'warning' | 'critical'
  message: string
  value: number | null
  fired_at: string
  resolved_at: string | null
}

export interface MetricPoint {
  timestamp: string
  value: number
  labels?: string
}

export interface MetricsTimeSeries {
  metric_type: string
  data_points: MetricPoint[]
}

export interface Setting {
  key: string
  value: string
}
