import { useState } from 'react'
import { useQuery } from '@tanstack/react-query'
import { probes } from '../../api/client'

type TimeRange = '1h' | '1d' | '3d'

const timeRangeOptions: { value: TimeRange; label: string }[] = [
  { value: '1h', label: '1 小时' },
  { value: '1d', label: '1 天' },
  { value: '3d', label: '3 天' },
]

const TOTAL_BAR_SLOTS = 30

export function UptimeBarRow({ probeId, defaultRange }: { probeId: number; defaultRange?: TimeRange }) {
  const [range, setRange] = useState<TimeRange>(defaultRange || '1h')

  const { data: bars = [] } = useQuery({
    queryKey: ['uptime-bars', probeId, range],
    queryFn: () => probes.uptimeBars(probeId, range),
    refetchInterval: 60000,
  })

  const paddedBars = [...bars]
  while (paddedBars.length < TOTAL_BAR_SLOTS) {
    paddedBars.push({ label: '', up: null })
  }

  return (
    <div className="px-4 py-3 flex flex-col gap-2">
      <div className="flex items-center justify-between">
        <span className="text-xs text-white">可用率</span>
        <div className="flex gap-1">
          {timeRangeOptions.map((opt) => (
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
      <div style={{ display: 'grid', gridTemplateColumns: `repeat(${TOTAL_BAR_SLOTS}, 1fr)`, gap: '2px' }}>
        {paddedBars.slice(0, TOTAL_BAR_SLOTS).map((bar, i) => {
          const color = bar.up === null ? 'bg-gray-700' : bar.up ? 'bg-ok' : 'bg-err'
          const status = bar.up === null ? '无数据' : bar.up ? '正常' : '异常'
          return (
            <div
              key={i}
              title={bar.label ? `${bar.label} - ${status}` : undefined}
              className={`h-4 rounded-[2px] ${color}`}
            />
          )
        })}
      </div>
    </div>
  )
}
