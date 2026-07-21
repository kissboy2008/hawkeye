import { useState } from 'react'
import { useQuery } from '@tanstack/react-query'
import { probes } from '../../api/client'
import { AreaChart, Area, XAxis, YAxis, Tooltip, ResponsiveContainer } from 'recharts'
import { toCSTString, toCSTFull } from '../../utils'

type ChartRange = '1h' | '1d' | '3d'

export function ResponseTimeChart({ probeId }: { probeId: number }) {
  const [range, setRange] = useState<ChartRange>('1h')

  const { data: chartData } = useQuery({
    queryKey: ['probe-response-time', probeId, range],
    queryFn: () => probes.responseTime(probeId, range),
    refetchInterval: 60000,
  })

  const safeChartData = chartData ?? []

  const { data: uptime24h } = useQuery({
    queryKey: ['probe-uptime-24h', probeId],
    queryFn: () => probes.uptimePercent(probeId, '24h'),
    refetchInterval: 60000,
  })

  const { data: uptime3d } = useQuery({
    queryKey: ['probe-uptime-3d', probeId],
    queryFn: () => probes.uptimePercent(probeId, '3d'),
    refetchInterval: 300000,
  })

  const chartRangeOptions: { value: ChartRange; label: string }[] = [
    { value: '1h', label: '1 小时' },
    { value: '1d', label: '1 天' },
    { value: '3d', label: '3 天' },
  ]

  return (
    <div className="px-3 md:px-4 py-3 flex flex-col gap-3">
      {/* Stat cards */}
      <div className="grid grid-cols-2 sm:grid-cols-4 gap-2">
        <div className="bg-bg rounded-lg p-2 text-center">
          <div className="text-[10px] text-white mb-0.5">在线(24h)</div>
          <div className={`text-sm font-semibold ${uptime24h && uptime24h.percent >= 99 ? 'text-ok' : uptime24h && uptime24h.percent >= 95 ? 'text-warn' : 'text-err'}`}>
            {uptime24h ? `${uptime24h.percent.toFixed(1)}%` : '-'}
          </div>
        </div>
        <div className="bg-bg rounded-lg p-2 text-center">
          <div className="text-[10px] text-white mb-0.5">在线(3d)</div>
          <div className={`text-sm font-semibold ${uptime3d && uptime3d.percent >= 99 ? 'text-ok' : uptime3d && uptime3d.percent >= 95 ? 'text-warn' : 'text-err'}`}>
            {uptime3d ? `${uptime3d.percent.toFixed(1)}%` : '-'}
          </div>
        </div>
        <div className="bg-bg rounded-lg p-2 text-center">
          <div className="text-[10px] text-white mb-0.5">平均响应({chartRangeOptions.find(o => o.value === range)?.label ?? range})</div>
          <div className="text-sm font-semibold text-white">
            {safeChartData.length > 0 ? `${Math.round(safeChartData.reduce((a: number, b: { latency_ms: number }) => a + b.latency_ms, 0) / safeChartData.length)}ms` : '-'}
          </div>
        </div>
        <div className="bg-bg rounded-lg p-2 text-center">
          <div className="text-[10px] text-white mb-0.5">当前状态</div>
          <div className={`text-sm font-semibold ${safeChartData.length > 0 && safeChartData[safeChartData.length - 1].status_code > 0 ? 'text-ok' : 'text-err'}`}>
            {safeChartData.length > 0 ? (safeChartData[safeChartData.length - 1].status_code > 0 ? `${safeChartData[safeChartData.length - 1].status_code}` : 'DOWN') : '-'}
          </div>
        </div>
      </div>

      {/* Chart */}
      <div className="flex items-center justify-between">
        <span className="text-xs text-white">响应时间</span>
        <div className="flex gap-1">
          {chartRangeOptions.map((opt) => (
            <button
              key={opt.value}
              onClick={(e) => { e.stopPropagation(); setRange(opt.value) }}
              className={`px-2 py-0.5 rounded text-[10px] transition-colors ${
                range === opt.value
                  ? 'bg-accent/20 text-accent'
                  : 'text-white hover:text-white hover:bg-bg-hover/50'
              }`}
            >
              {opt.label}
            </button>
          ))}
        </div>
      </div>
      {safeChartData.length > 0 ? (
        <div className="h-32">
          <ResponsiveContainer width="100%" height="100%">
            <AreaChart data={safeChartData} margin={{ top: 4, right: 4, left: -20, bottom: 0 }}>
              <defs>
                <linearGradient id={`grad-${probeId}`} x1="0" y1="0" x2="0" y2="1">
                  <stop offset="5%" stopColor="#22c55e" stopOpacity={0.3} />
                  <stop offset="95%" stopColor="#22c55e" stopOpacity={0} />
                </linearGradient>
              </defs>
              <XAxis
                dataKey="timestamp"
                tickFormatter={(v: string) => {
                  if (range === '3d') {
                    const d = parseInt(v.slice(8, 10), 10)
                    const h = (parseInt(v.slice(11, 13), 10) + 8) % 24
                    return `${parseInt(v.slice(5, 7), 10)}/${d} ${String(h).padStart(2, '0')}:${v.slice(14, 16)}`
                  }
                  return toCSTString(v)
                }}
                tick={{ fontSize: 10, fill: '#6b7280' }}
                interval="preserveStartEnd"
              />
              <YAxis
                tick={{ fontSize: 10, fill: '#6b7280' }}
                tickFormatter={(v: number) => `${v}ms`}
              />
              <Tooltip
                contentStyle={{ backgroundColor: '#1f2937', border: '1px solid #374151', borderRadius: '8px', fontSize: '12px' }}
                labelFormatter={(v: string) => toCSTFull(v)}
                formatter={(value: number, name: string) => [`${Math.round(value)}ms`, name === 'latency_ms' ? '响应时间' : name]}
              />
              <Area
                type="monotone"
                dataKey="latency_ms"
                stroke="#22c55e"
                fill={`url(#grad-${probeId})`}
                strokeWidth={1.5}
                dot={false}
              />
            </AreaChart>
          </ResponsiveContainer>
        </div>
      ) : (
        <div className="h-32 flex items-center justify-center text-white text-sm">暂无数据</div>
      )}
    </div>
  )
}
