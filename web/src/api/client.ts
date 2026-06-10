const BASE = '/api/v1'

function getToken(): string {
  return localStorage.getItem('auth_token') || ''
}

async function request<T>(path: string, options?: RequestInit): Promise<T> {
  const token = getToken()
  const headers: Record<string, string> = { 'Content-Type': 'application/json' }
  if (token) headers['Authorization'] = `Bearer ${token}`
  if (options?.headers) Object.assign(headers, options.headers)

  const res = await fetch(`${BASE}${path}`, { ...options, headers })
  if (res.status === 401) {
    localStorage.removeItem('auth_token')
    if (!path.startsWith('/auth/')) {
      window.location.reload()
    }
  }
  if (!res.ok) {
    const err = await res.json().catch(() => ({ error: res.statusText }))
    throw new Error(err.error || `HTTP ${res.status}`)
  }
  return res.json()
}

// Auth
export const auth = {
  check: () => request<{ need_register: boolean }>('/auth/check'),
  register: (username: string, password: string) =>
    request<{ message: string; username: string; token: string }>('/auth/register', {
      method: 'POST',
      body: JSON.stringify({ username, password }),
    }),
  login: (username: string, password: string) =>
    request<{ message: string; username: string; token: string }>('/auth/login', {
      method: 'POST',
      body: JSON.stringify({ username, password }),
    }),
}

// Agents
export const agents = {
  list: () => request<import('../types').Agent[]>('/agents'),
  get: (id: number) => request<import('../types').Agent>(`/agents/${id}`),
  create: (data: Partial<import('../types').Agent>) =>
    request<import('../types').Agent>('/agents', { method: 'POST', body: JSON.stringify(data) }),
  update: (id: number, data: Partial<import('../types').Agent>) =>
    request<import('../types').Agent>(`/agents/${id}`, { method: 'PUT', body: JSON.stringify(data) }),
  delete: (id: number) => request<{ message: string }>(`/agents/${id}`, { method: 'DELETE' }),
  test: (id: number) => request<{ success: boolean; error?: string }>(`/agents/${id}/test`, { method: 'POST' }),
  deleteMetrics: (id: number) => request<{ message: string; count: number }>(`/agents/${id}/metrics`, { method: 'DELETE' }),
  reorder: (ids: number[]) => request<{ message: string }>('/agents/reorder', { method: 'PUT', body: JSON.stringify({ ids }) }),
  pushServerURL: (id: number, server_url: string) =>
    request<{ status: string; message: string; db_saved: boolean }>(`/agents/${id}/server-url`, { method: 'PUT', body: JSON.stringify({ server_url }) }),
  pushAuthToken: (id: number, auth_token: string, old_token?: string) =>
    request<{ status: string; message: string; db_saved: boolean }>(`/agents/${id}/auth-token`, { method: 'PUT', body: JSON.stringify({ auth_token, old_token: old_token || '' }) }),
}

// Metrics
export const metrics = {
  latest: (agentId: number) => request<{ agent_id: number; timestamp: string; metrics: Record<string, string> }>(`/agents/${agentId}/metrics/latest`),
  timeRange: (agentId: number, type: string, from?: string, to?: string) =>
    request<import('../types').MetricsTimeSeries>(`/agents/${agentId}/metrics/${type}?from=${from || '1h'}&to=${to || ''}`),
}

// Probes
export const probes = {
  list: () => request<import('../types').WebProbe[]>('/probes'),
  get: (id: number) => request<import('../types').WebProbe>(`/probes/${id}`),
  create: (data: Partial<import('../types').WebProbe>) =>
    request<import('../types').WebProbe>('/probes', { method: 'POST', body: JSON.stringify(data) }),
  update: (id: number, data: Partial<import('../types').WebProbe>) =>
    request<import('../types').WebProbe>(`/probes/${id}`, { method: 'PUT', body: JSON.stringify(data) }),
  delete: (id: number) => request<{ message: string }>(`/probes/${id}`, { method: 'DELETE' }),
  check: (id: number) => request<import('../types').ProbeResult>(`/probes/${id}/check`, { method: 'POST' }),
  results: (id: number, limit = 100) => request<import('../types').ProbeResult[]>(`/probes/${id}/results?limit=${limit}`),
  stats: (id: number) => request<import('../types').ProbeStats>(`/probes/${id}/stats`),
  cert: (id: number) => request<import('../types').CertInfo | null>(`/probes/${id}/cert`),
  uptimeBars: (id: number, range: '1h' | '1d' | '3d' = '1h') =>
    request<import('../types').UptimeBar[]>(`/probes/${id}/uptime-bars?range=${range}`),
  responseTime: (id: number, range: '1h' | '1d' | '3d' = '1h') =>
    request<{ timestamp: string; latency_ms: number; status_code: number }[]>(`/probes/${id}/response-time?range=${range}`),
  uptimePercent: (id: number, range: '24h' | '3d' | '30d' = '24h') =>
    request<{ percent: number }>(`/probes/${id}/uptime-percent?range=${range}`),
  deleteResults: () => request<{ message: string; count: number }>('/probes/results', { method: 'DELETE' }),
  reorder: (ids: number[]) => request<{ message: string }>('/probes/reorder', { method: 'PUT', body: JSON.stringify({ ids }) }),
}

// Alert Rules
export const alertRules = {
  list: () => request<import('../types').AlertRule[]>('/alerts/rules'),
  get: (id: number) => request<import('../types').AlertRule>(`/alerts/rules/${id}`),
  create: (data: Partial<import('../types').AlertRule>) =>
    request<import('../types').AlertRule>('/alerts/rules', { method: 'POST', body: JSON.stringify(data) }),
  update: (id: number, data: Partial<import('../types').AlertRule>) =>
    request<import('../types').AlertRule>(`/alerts/rules/${id}`, { method: 'PUT', body: JSON.stringify(data) }),
  delete: (id: number) => request<{ message: string }>(`/alerts/rules/${id}`, { method: 'DELETE' }),
}

// Alert Events
export const alertEvents = {
  list: (resolved = true, limit = 50) =>
    request<import('../types').AlertEvent[]>(`/alerts/events?resolved=${resolved}&limit=${limit}`),
  testRule: (id: number) =>
    request<{ message: string }>(`/alerts/test/${id}`, { method: 'POST' }),
  testWebhook: (webhook: string) =>
    request<{ message: string }>('/alerts/test-webhook', { method: 'POST', body: JSON.stringify({ webhook }) }),
  clear: () =>
    request<{ message: string; count: number }>('/alerts/events', { method: 'DELETE' }),
}

// Version (public, no auth needed)
export const versionInfo = {
  get: () => request<{ version: string; agents: { id: number; name: string; version: string; online: boolean }[] }>('/version'),
}

// Settings
export const settings = {
  list: () => request<import('../types').Setting[]>('/settings'),
  update: (data: Record<string, string>) =>
    request<{ message: string }>('/settings', { method: 'PUT', body: JSON.stringify(data) }),
}

// Database
export const database = {
  info: () => request<{ size_bytes: number }>('/database/info'),
  purge: () => request<{ message: string; deleted_rows: number }>('/database/purge', { method: 'DELETE' }),
}

// Background images (custom uploads)
async function uploadRequest<T>(path: string, formData: FormData): Promise<T> {
  const token = getToken()
  const headers: Record<string, string> = {}
  if (token) headers['Authorization'] = `Bearer ${token}`

  const res = await fetch(`${BASE}${path}`, { method: 'POST', headers, body: formData })
  if (res.status === 401) {
    localStorage.removeItem('auth_token')
    window.location.reload()
  }
  if (!res.ok) {
    const err = await res.json().catch(() => ({ error: res.statusText }))
    throw new Error(err.error || `HTTP ${res.status}`)
  }
  return res.json()
}

export const bgImages = {
  upload: (file: File) => {
    const formData = new FormData()
    formData.append('image', file)
    return uploadRequest<{ filename: string; url: string }>('/bg/upload', formData)
  },
  list: () => request<{ images: { filename: string; url: string }[] }>('/bg/list'),
  delete: (filename: string) => request<{ message: string }>(`/bg/${encodeURIComponent(filename)}`, { method: 'DELETE' }),
}

// Widgets
export interface Widget {
  id: number
  name: string
  type: string
  url: string
  api_token: string
  node: string
  config: string
  description: string
  widget_group: string
  sort_order: number
  enabled: boolean
  created_at: string
  updated_at: string
}

export const widgets = {
  list: () => request<Widget[]>('/widgets'),
  create: (data: Partial<Widget>) =>
    request<Widget>('/widgets', { method: 'POST', body: JSON.stringify(data) }),
  update: (id: number, data: Partial<Widget>) =>
    request<Widget>(`/widgets/${id}`, { method: 'PUT', body: JSON.stringify(data) }),
  delete: (id: number) => request<{ message: string }>(`/widgets/${id}`, { method: 'DELETE' }),
  data: (id: number) => request<any>(`/widgets/${id}/data`),
  reorder: (ids: number[]) =>
    request<{ message: string }>('/widgets/reorder', { method: 'PUT', body: JSON.stringify({ ids }) }),
  renameGroup: (oldName: string, newName: string) =>
    request<{ message: string }>('/widgets/rename-group', { method: 'PUT', body: JSON.stringify({ old_name: oldName, new_name: newName }) }),
  move: (id: number, widgetGroup: string) =>
    request<{ message: string }>('/widgets/move', { method: 'PUT', body: JSON.stringify({ id, widget_group: widgetGroup }) }),
  openclashNodes: (id: number) =>
    request<{ current: string; nodes: string[] }>(`/widgets/${id}/openclash-nodes`),
  openclashSwitch: (id: number, node: string) =>
    request<{ message: string; node: string }>(`/widgets/${id}/openclash-switch`, { method: 'PUT', body: JSON.stringify({ node }) }),
  openclashControl: (id: number, action: 'start' | 'stop' | 'restart') =>
    request<{ message: string; action: string; output: string }>(`/widgets/${id}/openclash-control`, { method: 'POST', body: JSON.stringify({ action }) }),
  openclashStatus: (id: number) =>
    request<{ running: boolean }>(`/widgets/${id}/openclash-status`),
}
