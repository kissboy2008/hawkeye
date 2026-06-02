import { useState } from 'react'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { useNavigate } from 'react-router-dom'
import { useSortable } from '@dnd-kit/sortable'
import { CSS } from '@dnd-kit/utilities'
import { probes } from '../../api/client'
import type { WebProbe } from '../../types'
import { DragHandle } from './DragHandle'
import { CertBadge } from './CertBadge'
import { UptimeBarRow } from './UptimeBarRow'
import { ResponseTimeChart } from './ResponseTimeChart'

interface SortableProbeCardProps {
  probe: WebProbe
  onEdit: (probe: WebProbe) => void
  onDelete: (id: number) => void
  defaultExpanded?: boolean
}

export function SortableProbeCard({ probe, onEdit, onDelete, defaultExpanded }: SortableProbeCardProps) {
  const navigate = useNavigate()
  const queryClient = useQueryClient()
  const [expanded, setExpanded] = useState(defaultExpanded || false)
  const {
    attributes,
    listeners,
    setNodeRef,
    transform,
    transition,
    isDragging,
  } = useSortable({ id: probe.id })

  const style = {
    transform: CSS.Transform.toString(transform),
    transition,
    opacity: isDragging ? 0.5 : 1,
    zIndex: isDragging ? 10 : undefined,
  }

  const { data: results } = useQuery({
    queryKey: ['probe-results', probe.id],
    queryFn: () => probes.results(probe.id, 1),
    refetchInterval: 15000,
  })

  const safeResults = results ?? []

  const { data: stats } = useQuery({
    queryKey: ['probe-stats', probe.id],
    queryFn: () => probes.stats(probe.id),
    refetchInterval: 60000,
  })

  const { data: cert } = useQuery({
    queryKey: ['probe-cert', probe.id],
    queryFn: () => probes.cert(probe.id),
    refetchInterval: 300000,
  })

  const checkOne = useMutation({
    mutationFn: (id: number) => probes.check(id),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: ['probe-results', probe.id] }),
  })

  const lastResult = safeResults.length > 0 ? safeResults[safeResults.length - 1] : null

  return (
    <div
      ref={setNodeRef}
      style={style}
      {...attributes}
      className={`group bg-bg-card rounded-xl border border-purple-500/10 overflow-hidden ${isDragging ? 'shadow-lg shadow-accent/10' : ''} ${!probe.enabled ? 'opacity-50' : ''}`}
    >
      {/* Main row */}
      <div
        className="flex items-center gap-3 px-4 py-4 cursor-pointer hover:bg-bg-hover/30 transition-colors"
        onClick={() => setExpanded(!expanded)}
      >
        <DragHandle listeners={listeners} />
        {/* Status dot */}
        <span className={`w-2.5 h-2.5 rounded-full flex-shrink-0 ${lastResult ? (lastResult.success ? 'bg-ok' : 'bg-err') : 'bg-gray-600'}`} />

        {/* Name & URL */}
        <div className="flex-1 min-w-0">
          <div className="flex items-center gap-2">
            <span className="font-medium text-sm">{probe.name}</span>
            {cert && <CertBadge cert={cert} />}
          </div>
          <span className="text-xs text-gray-500 font-mono truncate block">{probe.url}</span>
        </div>

        {/* Quick stats */}
        {stats && (
          <div className="flex items-center gap-4 text-xs flex-shrink-0">
            <div className="text-center">
              <div className={`font-semibold ${stats.up_percent >= 99 ? 'text-ok' : stats.up_percent >= 95 ? 'text-warn' : 'text-err'}`}>
                {stats.up_percent.toFixed(1)}%
              </div>
              <div className="text-gray-500">可用率</div>
            </div>
            <div className="text-center">
              <div className="font-semibold text-gray-300">{Math.round(stats.avg_latency_ms)}ms</div>
              <div className="text-gray-500">平均延迟</div>
            </div>
          </div>
        )}

        {/* Last status code / disabled badge */}
        <div className="flex items-center gap-1 text-xs flex-shrink-0">
          {!probe.enabled ? (
            <span className="text-gray-500 text-[10px] px-1.5 py-0.5 rounded bg-gray-700/50">已禁用</span>
          ) : lastResult ? (
            <span className={`font-mono ${lastResult.success ? 'text-ok' : 'text-err'}`}>
              {lastResult.success ? `${lastResult.status_code}` : 'DOWN'}
            </span>
          ) : null}
        </div>

        {/* Expand arrow */}
        <svg
          className={`w-4 h-4 text-gray-500 flex-shrink-0 transition-transform duration-200 ${expanded ? 'rotate-180' : ''}`}
          fill="none" stroke="currentColor" viewBox="0 0 24 24"
        >
          <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M19 9l-7 7-7-7" />
        </svg>
      </div>

      {/* Uptime bar row */}
      <div className="border-t border-purple-500/10/50">
        <UptimeBarRow probeId={probe.id} />
      </div>

      {/* Expanded content - response time chart */}
      {expanded && (
        <div className="border-t border-purple-500/10/50">
          <ResponseTimeChart probeId={probe.id} />
        </div>
      )}

      {/* Action buttons */}
      <div className="border-t border-purple-500/10/50 px-4 py-2 flex items-center justify-end gap-2">
        <button
          onClick={(e) => { e.stopPropagation(); navigate(`/probe/${probe.id}`) }}
          className="px-3 py-1 rounded-lg text-xs text-accent bg-accent/10 hover:bg-accent/20 transition-colors"
        >
          详情
        </button>
        <button
          onClick={(e) => { e.stopPropagation(); checkOne.mutate(probe.id) }}
          className="px-3 py-1 rounded-lg text-xs bg-accent text-black font-medium hover:bg-accent/80 transition-colors"
          disabled={checkOne.isPending}
        >
          {checkOne.isPending ? '检测中...' : 'Ping'}
        </button>
        <button
          onClick={(e) => { e.stopPropagation(); onEdit(probe) }}
          className="px-3 py-1.5 rounded-lg text-xs text-accent bg-accent/10 hover:bg-accent/20 transition-colors"
        >
          编辑
        </button>
        <button
          onClick={(e) => { e.stopPropagation(); if (confirm('确定删除此探测？')) onDelete(probe.id) }}
          className="px-3 py-1.5 rounded-lg text-xs text-err bg-err/10 hover:bg-err/20 transition-colors"
        >
          删除
        </button>
      </div>
    </div>
  )
}
