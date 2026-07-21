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

/** Full CST datetime, e.g. "6/8 01:05". */
export function toCSTFull(ts: string): string {
  const month = parseInt(ts.slice(5, 7), 10)
  const day = parseInt(ts.slice(8, 10), 10)
  const hour = parseInt(ts.slice(11, 13), 10)
  const min = ts.slice(14, 16)
  let cstHour = hour + 8
  let cstDay = day
  let cstMonth = month
  if (cstHour >= 24) {
    cstHour -= 24
    cstDay += 1
  }
  return `${cstMonth}/${cstDay} ${String(cstHour).padStart(2, '0')}:${min}`
}
