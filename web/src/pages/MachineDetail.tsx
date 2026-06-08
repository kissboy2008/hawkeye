import { useState, useMemo } from 'react'
import { useParams, Link } from 'react-router-dom'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { agents, metrics } from '../api/client'
import { toCSTString } from '../utils'
import {
  LineChart, Line, XAxis, YAxis, CartesianGrid, Tooltip, ResponsiveContainer,
} from 'recharts'

const TIME_RANGES = [
  { label: '1小时', value: '1h' },
  { label: '1天', value: '24h' },
  { label: '7天', value: '168h' },
]

function parseJsonSafe(str: string): any {
  try { return JSON.parse(str) } catch { return null }
}

function extractValue(data: string, key: string): number {
  const obj = parseJsonSafe(data)
  if (!obj) return 0
  if (Array.isArray(obj)) return obj[0]?.[key] ?? 0
  return obj[key] ?? 0
}

function MetricChart({ agentId, type, from, label, color, dataKey = 'usage_percent' }: {
  agentId: number; type: string; from: string; label: string; color: string; dataKey?: string
}) {
  const { data } = useQuery({
    queryKey: ['metrics', agentId, type, from],
    queryFn: () => metrics.timeRange(agentId, type, from),
    refetchInterval: 30000,
    enabled: !!agentId,
  })

  const chartData = useMemo(() => {
    if (!data?.data_points) return []
    return data.data_points.map((p) => ({
      time: toCSTString(p.timestamp),
      value: Math.round((p.value || extractValue(p.labels || '{}', dataKey)) * 10) / 10,
    }))
  }, [data, dataKey])

  if (!chartData.length) {
    return (
      <div className="bg-bg-card/70 rounded-xl p-4">
        <h3 className="text-sm font-medium mb-2">{label}</h3>
        <div className="h-48 flex items-center justify-center text-gray-500 text-sm">暂无数据</div>
      </div>
    )
  }

  return (
    <div className="bg-bg-card/70 rounded-xl p-4">
      <h3 className="text-sm font-medium mb-3">{label}</h3>
      <ResponsiveContainer width="100%" height={240}>
        <LineChart data={chartData}>
          <CartesianGrid strokeDasharray="3 3" stroke="#334155" />
          <XAxis dataKey="time" tick={{ fontSize: 11, fill: '#64748b' }} />
          <YAxis domain={[0, 100]} tick={{ fontSize: 11, fill: '#64748b' }} />
          <Tooltip
            contentStyle={{ background: '#1e293b', border: '1px solid #334155', borderRadius: 8, fontSize: 12 }}
            labelStyle={{ color: '#94a3b8' }}
            formatter={(val: number) => [val.toFixed(1), '值']}
          />
          <Line type="monotone" dataKey="value" stroke={color} dot={false} strokeWidth={2} />
        </LineChart>
      </ResponsiveContainer>
    </div>
  )
}

export default function MachineDetail() {
  const { name } = useParams<{ name: string }>()
  const agentName = decodeURIComponent(name || '')
  const [timeRange, setTimeRange] = useState('1h')
  const queryClient = useQueryClient()

  // Step 1: Resolve name → agent (with id)
  const { data: agentList = [], isLoading: listLoading } = useQuery({
    queryKey: ['agents'],
    queryFn: agents.list,
  })
  const agent = agentList.find((a) => a.name === agentName)
  const agentId = agent?.id ?? 0

  const deleteData = useMutation({
    mutationFn: () => agents.deleteMetrics(agentId),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: ['metrics', agentId] }),
  })

  const { data: latest } = useQuery({
    queryKey: ['latest', agentId],
    queryFn: () => metrics.latest(agentId),
    enabled: !!agentId,
    refetchInterval: 15000,
  })

  const cpu = parseJsonSafe(latest?.metrics?.cpu || '')
  const mem = parseJsonSafe(latest?.metrics?.memory || '')

  // Loading state
  if (listLoading) {
    return (
      <div className="py-4 px-2 md:py-6 md:px-3 max-w-[1920px] mx-auto">
        <div className="py-16 text-center text-gray-500">加载中...</div>
      </div>
    )
  }

  // Not found
  if (!agent) {
    return (
      <div className="py-4 px-2 md:py-6 md:px-3 max-w-[1920px] mx-auto">
        <div className="flex items-center gap-2 mb-4">
          <Link to="/" className="text-gray-500 hover:text-gray-300">&larr;</Link>
          <h2 className="text-xl font-bold">主机未找到</h2>
        </div>
        <p className="text-gray-500">找不到名为「{agentName}」的主机</p>
      </div>
    )
  }

  return (
    <div className="py-4 px-2 md:py-6 md:px-3 max-w-[1920px] mx-auto">
      <div className="flex items-center justify-between mb-4 md:mb-6">
        <div className="flex items-center gap-2 md:gap-3 min-w-0">
          <Link to="/" className="text-gray-500 hover:text-gray-300 shrink-0">&larr;</Link>
          <div className="min-w-0">
            <h2 className="text-xl md:text-2xl font-bold truncate">{agent.name}</h2>
            <p className="text-gray-500 text-xs md:text-sm font-mono truncate">{agent.address}</p>
          </div>
        </div>
        <button
          onClick={() => { if (confirm('确定删除此主机的监控数据？此操作不可撤销。')) deleteData.mutate() }}
          disabled={deleteData.isPending}
          className="px-3 py-1.5 rounded-lg text-xs text-err border border-err/40 hover:bg-err/10 transition-colors disabled:opacity-50 flex items-center gap-1.5"
        >
          <svg className="w-3.5 h-3.5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
            <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M19 7l-.867 12.142A2 2 0 0116.138 21H7.862a2 2 0 01-1.995-1.858L5 7m5 4v6m4-6v6m1-10V4a1 1 0 00-1-1h-4a1 1 0 00-1 1v3M4 7h16" />
          </svg>
          {deleteData.isPending ? '删除中...' : '清除数据'}
        </button>
      </div>

      {/* Time Range Selector */}
      <div className="flex gap-2 mb-6">
        {TIME_RANGES.map((r) => (
          <button
            key={r.value}
            onClick={() => setTimeRange(r.value)}
            className={`px-3 py-1.5 rounded-lg text-sm transition-colors ${
              timeRange === r.value ? 'bg-accent text-black' : 'bg-bg-card/70 text-gray-400 hover:text-gray-200'
            }`}
          >
            {r.label}
          </button>
        ))}
      </div>

      {/* Current Stats */}
      <div className="grid grid-cols-2 gap-3 mb-6">
        {cpu && (
          <div className="bg-bg-card/70 rounded-lg p-3">
            <div className="text-xs text-gray-500">CPU 使用率</div>
            <div className="text-xl font-bold mt-1">{cpu.usage_percent.toFixed(1)}%</div>
            <div className="text-xs text-gray-500">{cpu.cores} 核心</div>
          </div>
        )}
        {mem && (
          <div className="bg-bg-card/70 rounded-lg p-3">
            <div className="text-xs text-gray-500">内存使用率</div>
            <div className="text-xl font-bold mt-1">{mem.usage_percent.toFixed(1)}%</div>
            <div className="text-xs text-gray-500">{mem.used_mb} / {mem.total_mb} MB</div>
          </div>
        )}
      </div>

      {/* Charts */}
      <div className="space-y-4">
        <MetricChart agentId={agentId} type="cpu" from={timeRange} label="CPU 使用率" color="#38bdf8" />
        <MetricChart agentId={agentId} type="memory" from={timeRange} label="内存使用率" color="#a78bfa" />
      </div>
    </div>
  )
}
