import { useQuery } from '@tanstack/react-query'
import { metrics } from '../api/client'
import type { Agent } from '../types'
import { safeJsonParse, type CpuMetric, type MemMetric, type UptimeMetric } from '../utils'

export interface AgentMetricsResult {
  cpu: CpuMetric | null
  mem: MemMetric | null
  uptimeData: UptimeMetric | null
  hostname: string
  cpuPct: number
  memPct: number
  cores: number
  memUsedGb: string
  memTotalGb: string
  load1: number
  load5: number
  load15: number
  hasLoad: boolean
  modelName: string
  kernelVersion: string
  uptimeS: number
}

export function useAgentMetrics(agent: Agent): AgentMetricsResult {
  const { data: latest } = useQuery({
    queryKey: ['latest', agent.id],
    queryFn: () => metrics.latest(agent.id),
    enabled: agent.status === 'online',
  })

  const cpu = latest?.metrics?.cpu ? safeJsonParse<CpuMetric>(latest.metrics.cpu) : null
  const mem = latest?.metrics?.memory ? safeJsonParse<MemMetric>(latest.metrics.memory) : null
  const uptimeData = latest?.metrics?.uptime ? safeJsonParse<UptimeMetric>(latest.metrics.uptime) : null

  return {
    cpu,
    mem,
    uptimeData,
    hostname: agent.address,
    cpuPct: cpu?.usage_percent ?? 0,
    memPct: mem?.usage_percent ?? 0,
    cores: cpu?.cores ?? 1,
    memUsedGb: mem ? (mem.used_mb / 1024).toFixed(1) : '-',
    memTotalGb: mem ? (mem.total_mb / 1024).toFixed(1) : '-',
    load1: cpu?.load1 ?? 0,
    load5: cpu?.load5 ?? 0,
    load15: cpu?.load15 ?? 0,
    hasLoad: (cpu?.load1 ?? 0) > 0 || (cpu?.load5 ?? 0) > 0 || (cpu?.load15 ?? 0) > 0,
    modelName: cpu?.model_name || '',
    kernelVersion: cpu?.kernel_version || '',
    uptimeS: uptimeData?.uptime_seconds || 0,
  }
}
