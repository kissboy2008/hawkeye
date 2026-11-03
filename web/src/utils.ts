/** Parse UTC timestamp and format as CST (UTC+8). */
export function toCSTString(ts: string): string {
  const hour = parseInt(ts.slice(11, 13), 10)
  const min = ts.slice(14, 16)
  const cstHour = (hour + 8) % 24
  return `${String(cstHour).padStart(2, '0')}:${min}`
}

export interface CpuMetric {
  usage_percent: number
  load1?: number
  load5?: number
  load15?: number
  cores?: number
  model_name?: string
  kernel_version?: string
}

export interface MemMetric {
  usage_percent: number
  used_mb: number
  total_mb: number
}

export interface UptimeMetric {
  uptime_seconds: number
}

/** Safely parse JSON, returning null on any error instead of throwing. */
export function safeJsonParse<T = unknown>(json: string): T | null {
  try {
    return JSON.parse(json) as T
  } catch {
    return null
  }
}

/** Format relative time like "3分钟", "2小时", "1天". */
export function formatTimeSince(date: Date): string {
  const seconds = Math.floor((Date.now() - date.getTime()) / 1000)
  if (seconds < 60) return '刚刚'
  const minutes = Math.floor(seconds / 60)
  if (minutes < 60) return `${minutes}分钟`
  const hours = Math.floor(minutes / 60)
  if (hours < 24) return `${hours}小时`
  const days = Math.floor(hours / 24)
  return `${days}天`
}

/**
 * Full CST datetime, e.g. "6/8 01:05".
 * 兼容两种输入：DB 里的 UTC 无时区串（"2026-11-02T22:03:16..."）
 * 和带偏移的 RFC3339（"2026-11-02T22:03:16+08:00"），后者不再二次加 8 小时。
 */
export function toCSTFull(ts: string): string {
  if (!ts) return '—'
  const hasZone = /(?:Z|[+-]\d{2}:?\d{2})$/.test(ts)
  const normalized = ts.includes(' ') && !hasZone ? ts.replace(' ', 'T') : ts
  const base = new Date(hasZone ? normalized : `${normalized}Z`)
  if (isNaN(base.getTime())) return ts
  const cst = new Date(base.getTime() + 8 * 3600 * 1000)
  const month = cst.getUTCMonth() + 1
  const day = cst.getUTCDate()
  const hour = String(cst.getUTCHours()).padStart(2, '0')
  const min = String(cst.getUTCMinutes()).padStart(2, '0')
  return `${month}/${day} ${hour}:${min}`
}
