import { useState } from 'react'
import { useParams, useNavigate } from 'react-router-dom'
import { useQuery, useMutation } from '@tanstack/react-query'
import { LineChart, Line, XAxis, YAxis, Tooltip, ResponsiveContainer, CartesianGrid, Area, AreaChart } from 'recharts'
import { probes } from '../api/client'

type TimeRange = '1h' | '1d' | '7d'
type UptimeRange = '1h' | '1d' | '7d'

const timeRangeOptions: { value: TimeRange; label: string }[] = [
  { value: '1h', label: '1 小时' },
  { value: '1d', label: '1 天' },
  { value: '7d', label: '7 天' },
]

export default function ProbeDetail() {
  const { id } = useParams<{ id: string }>()
  const navigate = useNavigate()
  const probeId = Number(id)
  const [chartRange, setChartRange] = useState<TimeRange>('1h')

  const { data: probe } = useQuery({
    queryKey: ['probe', probeId],
    queryFn: () => probes.get(probeId),
    enabled: !!probeId,
  })

  const { data: stats } = useQuery({
    queryKey: ['probe-stats', probeId],
    queryFn: () => probes.stats(probeId),
    refetchInterval: 60000,
    enabled: !!probeId,
  })

  const { data: results } = useQuery({
    queryKey: ['probe-results', probeId],
    queryFn: () => probes.results(probeId, 1),
    refetchInterval: 15000,
    enabled: !!probeId,
  })

  const { data: bars } = useQuery({
    queryKey: ['uptime-bars', probeId, '1d'],
    queryFn: () => probes.uptimeBars(probeId, '1d'),
    refetchInterval: 60000,
    enabled: !!probeId,
  })

  const { data: uptime24h } = useQuery({
    queryKey: ['uptime-percent', probeId, '24h'],
    queryFn: () => probes.uptimePercent(probeId, '24h'),
    refetchInterval: 60000,
    enabled: !!probeId,
  })

  const { data: uptime30d } = useQuery({
    queryKey: ['uptime-percent', probeId, '30d'],
    queryFn: () => probes.uptimePercent(probeId, '30d'),
    refetchInterval: 60000,
    enabled: !!probeId,
  })

  const { data: cert } = useQuery({
    queryKey: ['probe-cert', probeId],
    queryFn: () => probes.cert(probeId),
    refetchInterval: 300000,
    enabled: !!probeId,
  })

  const { data: chartData } = useQuery({
    queryKey: ['response-time', probeId, chartRange],
    queryFn: () => probes.responseTime(probeId, chartRange),
    refetchInterval: 60000,
    enabled: !!probeId,
  })

  const safeResults = results ?? []
  const safeBars = bars ?? []
  const safeChartData = chartData ?? []

  const checkOne = useMutation({
    mutationFn: (pid: number) => probes.check(pid),
  })

  const lastResult = safeResults.length > 0 ? safeResults[safeResults.length - 1] : null
  const isUp = lastResult ? lastResult.success : null

  if (!probe) {
    return (
      <div className="p-4 md:p-6 max-w-[1800px] mx-auto">
        <div className="py-20 text-center text-gray-500">加载中...</div>
      </div>
    )
  }

  // Prepare chart data
  const chartPoints = safeChartData.map((p) => ({
    time: p.timestamp.slice(11, 16),
    latency: Math.round(p.latency_ms),
    isDown: p.status_code === 0 || p.status_code >= 400,
  }))

  const currentLatency = lastResult ? Math.round(lastResult.latency_ms) : 0
  const avgLatency24h = stats ? Math.round(stats.avg_latency_ms) : 0
  const uptime24hValue = uptime24h ? uptime24h.percent : 100
  const uptime30dValue = uptime30d ? uptime30d.percent : 100

  return (
    <div className="p-4 md:p-6 max-w-[1800px] mx-auto">
      <button
        onClick={() => navigate(-1)}
        className="flex items-center gap-1 text-gray-400 hover:text-gray-200 text-sm mb-4 transition-colors"
      >
        <svg className="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
          <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M15 19l-7-7 7-7" />
        </svg>
        返回
      </button>

      {/* Header */}
      <div className="flex flex-col sm:flex-row sm:items-center justify-between gap-2 mb-4">
        <div className="flex items-center gap-2 md:gap-3 min-w-0">
          <h2 className="text-xl font-bold truncate">{probe.name}</h2>
          <span className={`text-xs px-2.5 py-1 rounded-full font-medium shrink-0 ${
            isUp === true ? 'bg-ok/15 text-ok' : isUp === false ? 'bg-err/15 text-err' : 'bg-gray-700 text-gray-400'
          }`}>
            {isUp === true ? '正常' : isUp === false ? '异常' : '未知'}
          </span>
        </div>
        <div className="flex items-center gap-2">
          <span className="text-xs text-gray-500">每 {probe.interval_s}s 检测</span>
          <button
            onClick={() => checkOne.mutate(probeId)}
            disabled={checkOne.isPending}
            className="px-3 py-1.5 rounded-lg text-xs bg-accent text-black font-medium hover:bg-accent/80 transition-colors disabled:opacity-50"
          >
            {checkOne.isPending ? '检测中...' : 'Ping'}
          </button>
        </div>
      </div>

      {/* Uptime bars - pill style */}
      <div className="bg-bg-card rounded-xl border border-purple-500/10 p-4 mb-4">
        <div className="flex items-center justify-between mb-3">
          <span className="text-xs text-gray-500">过去 24 小时</span>
        </div>
        <div className="grid grid-cols-[repeat(30,1fr)] gap-[3px]">
          {safeBars.length > 0 ? safeBars.map((bar, i) => {
            const color = bar.up === null ? 'bg-gray-700' : bar.up ? 'bg-ok' : 'bg-err'
            const status = bar.up === null ? '无数据' : bar.up ? '正常' : '异常'
            return (
              <div
                key={i}
                title={`${bar.label} - ${status}`}
                className={`h-2.5 rounded-full ${color}`}
              />
            )
          }) : Array.from({ length: 30 }).map((_, i) => (
            <div key={i} className="h-2.5 rounded-full bg-gray-700" title="无数据" />
          ))}
        </div>
      </div>

      {/* Stat cards */}
      <div className="grid grid-cols-2 sm:grid-cols-3 lg:grid-cols-5 gap-3 mb-4">
        <StatCard
          label="响应"
          value={`${currentLatency}ms`}
          color="text-gray-200"
        />
        <StatCard
          label="平均响应 (24h)"
          value={`${avgLatency24h}ms`}
          color="text-gray-200"
        />
        <StatCard
          label="在线时间 (24h)"
          value={`${uptime24hValue.toFixed(2)}%`}
          color={uptime24hValue >= 99 ? 'text-ok' : uptime24hValue >= 95 ? 'text-warn' : 'text-err'}
        />
        <StatCard
          label="在线时间 (30d)"
          value={`${uptime30dValue.toFixed(2)}%`}
          color={uptime30dValue >= 99 ? 'text-ok' : uptime30dValue >= 95 ? 'text-warn' : 'text-err'}
        />
        <StatCard
          label="证书有效期"
          value={cert ? `${cert.days_left} 天` : 'N/A'}
          color={!cert ? 'text-gray-500' : cert.days_left <= 7 ? 'text-err' : cert.days_left <= 30 ? 'text-warn' : 'text-ok'}
        />
      </div>

      {/* Response time chart */}
      <div className="bg-bg-card rounded-xl border border-purple-500/10 p-4">
        <div className="flex items-center justify-between mb-4">
          <span className="text-sm font-medium">响应时间</span>
          <div className="flex gap-1 bg-bg rounded-lg p-0.5">
            {timeRangeOptions.map((opt) => (
              <button
                key={opt.value}
                onClick={() => setChartRange(opt.value)}
                className={`px-2.5 py-1 rounded-md text-xs transition-colors ${
                  chartRange === opt.value
                    ? 'bg-accent/20 text-accent'
                    : 'text-gray-500 hover:text-gray-300'
                }`}
              >
                {opt.label}
              </button>
            ))}
          </div>
        </div>
        {chartPoints.length > 0 ? (
          <div className="h-48">
            <ResponsiveContainer width="100%" height="100%">
              <AreaChart data={chartPoints}>
                <defs>
                  <linearGradient id="latencyGradient" x1="0" y1="0" x2="0" y2="1">
                    <stop offset="5%" stopColor="#38bdf8" stopOpacity={0.3} />
                    <stop offset="95%" stopColor="#38bdf8" stopOpacity={0} />
                  </linearGradient>
                </defs>
                <CartesianGrid strokeDasharray="3 3" stroke="#334155" />
                <XAxis
                  dataKey="time"
                  tick={{ fill: '#64748b', fontSize: 10 }}
                  axisLine={{ stroke: '#334155' }}
                  tickLine={false}
                  interval="preserveStartEnd"
                />
                <YAxis
                  tick={{ fill: '#64748b', fontSize: 10 }}
                  axisLine={{ stroke: '#334155' }}
                  tickLine={false}
                  width={45}
                  tickFormatter={(v) => `${v}ms`}
                />
                <Tooltip
                  contentStyle={{
                    backgroundColor: '#1e293b',
                    border: '1px solid #334155',
                    borderRadius: '8px',
                    fontSize: '12px',
                    color: '#e2e8f0',
                  }}
                  formatter={(value: number) => [`${value}ms`, '响应时间']}
                  labelFormatter={(label) => `时间: ${label}`}
                />
                <Area
                  type="monotone"
                  dataKey="latency"
                  stroke="#38bdf8"
                  strokeWidth={2}
                  fill="url(#latencyGradient)"
                  dot={false}
                  activeDot={{ r: 4, fill: '#38bdf8' }}
                />
              </AreaChart>
            </ResponsiveContainer>
          </div>
        ) : (
          <div className="h-48 flex items-center justify-center text-gray-500 text-sm">
            暂无数据
          </div>
        )}
      </div>
    </div>
  )
}

function StatCard({ label, value, color }: { label: string; value: string; color: string }) {
  return (
    <div className="bg-bg-card rounded-xl border border-purple-500/10 p-3">
      <div className={`text-xl font-bold ${color}`}>{value}</div>
      <div className="text-xs text-gray-500 mt-0.5">{label}</div>
    </div>
  )
}
