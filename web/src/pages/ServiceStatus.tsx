import { useState } from 'react'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { useSearchParams } from 'react-router-dom'
import { probes } from '../api/client'
import type { WebProbe } from '../types'
import { ProbeCard } from '../components/probe/ProbeCard'

export default function ServiceStatus() {
  const queryClient = useQueryClient()
  const { data: list = [] } = useQuery({ queryKey: ['probes'], queryFn: probes.list, refetchInterval: 30000 })
  const [searchParams] = useSearchParams()
  const expandedProbeId = searchParams.get('probe')
  const [editing, setEditing] = useState<Partial<WebProbe> | null>(null)

  const update = useMutation({
    mutationFn: ({ id, ...data }: Partial<WebProbe> & { id: number }) => probes.update(id, data),
    onSuccess: () => { queryClient.invalidateQueries({ queryKey: ['probes'] }); setEditing(null) },
  })
  const remove = useMutation({
    mutationFn: (id: number) => probes.delete(id),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: ['probes'] }),
  })
  const handleDelete = (id: number) => remove.mutate(id)

  // Summary stats
  const totalProbes = list.filter((p) => p.enabled).length
  const { data: allStats } = useQuery({
    queryKey: ['probe-stats-all'],
    queryFn: async () => {
      const enabled = list.filter((p) => p.enabled)
      const stats = await Promise.all(enabled.map((p) => probes.stats(p.id)))
      const total = stats.reduce((a, s) => a + s.total_checks, 0)
      const success = stats.reduce((a, s) => a + s.success_count, 0)
      const avgLatency = stats.length > 0
        ? stats.reduce((a, s) => a + s.avg_latency_ms, 0) / stats.length
        : 0
      const upPercent = total > 0 ? (success / total) * 100 : 100
      return { total, success, avgLatency, upPercent }
    },
    enabled: totalProbes > 0,
    refetchInterval: 30000,
  })

  const handleEdit = (probe: WebProbe) => setEditing(probe)

  const handleSubmit = (e: React.FormEvent) => {
    e.preventDefault()
    if (!editing || !editing.id) return
    update.mutate({ id: editing.id, name: editing.name!, url: editing.url!, method: editing.method || 'GET', expected_status: editing.expected_status || 200, timeout_ms: editing.timeout_ms || 5000, interval_s: editing.interval_s || 60, enabled: editing.enabled !== false })
  }

  return (
    <div className="py-4 px-2 md:py-6 md:px-3 max-w-[1920px] mx-auto">
      {/* Header */}
      <div className="mb-4 md:mb-6">
        <h2 className="page-title">网站状态</h2>
        <p className="text-fg-3 text-sm">网站监控 & SSL 证书追踪</p>
      </div>

      {/* Summary cards */}
      {allStats && (
        <div className="grid grid-cols-2 sm:grid-cols-4 gap-3 mb-6">
          <div className="bg-bg-card/70 rounded-xl p-3 flex flex-col items-center justify-center">
            <div className="text-xs text-fg-3 mb-1">监控服务数</div>
            <div className="text-xl font-bold">{totalProbes}</div>
          </div>
          <div className="bg-bg-card/70 rounded-xl p-3 flex flex-col items-center justify-center">
            <div className="text-xs text-fg-3 mb-1">总检测次数</div>
            <div className="text-xl font-bold">{allStats.total}</div>
          </div>
          <div className="bg-bg-card/70 rounded-xl p-3 flex flex-col items-center justify-center">
            <div className="text-xs text-fg-3 mb-1">整体可用率</div>
            <div className={`text-xl font-bold ${allStats.upPercent >= 99 ? 'text-ok' : allStats.upPercent >= 95 ? 'text-warn' : 'text-err'}`}>
              {allStats.upPercent.toFixed(1)}%
            </div>
          </div>
          <div className="bg-bg-card/70 rounded-xl p-3 flex flex-col items-center justify-center">
            <div className="text-xs text-fg-3 mb-1">平均响应</div>
            <div className="text-xl font-bold text-fg-2">{Math.round(allStats.avgLatency)}ms</div>
          </div>
        </div>
      )}

      {/* Probe list */}
      <div className="space-y-3">
        {list.map((probe) => (
          <ProbeCard
            key={probe.id}
            probe={probe}
            onEdit={handleEdit}
            onDelete={handleDelete}
            defaultExpanded={String(probe.id) === expandedProbeId}
          />
        ))}

        {list.length === 0 && (
          <div className="py-16 text-center text-fg-3">
            <div className="text-4xl mb-3">📡</div>
            <div>暂无监控服务</div>
          </div>
        )}
      </div>

      {/* Modal */}
      {editing && (
        <div className="fixed inset-0 bg-black/50 flex items-center justify-center z-50">
          <div className="bg-bg-card/70 rounded-xl p-4 md:p-6 w-[calc(100%-2rem)] max-w-md mx-4">
            <h3 className="text-lg font-semibold mb-4">编辑探测</h3>
            <form onSubmit={handleSubmit} className="space-y-3">
              <div>
                <label className="text-xs text-fg-3 block mb-1">名称</label>
                <input
                  className="w-full bg-bg px-3 py-2 rounded-lg border text-sm focus:border-accent outline-none"
                  value={editing.name || ''} onChange={(e) => setEditing({ ...editing, name: e.target.value })} required
                />
              </div>
              <div>
                <label className="text-xs text-fg-3 block mb-1">URL</label>
                <input
                  className="w-full bg-bg px-3 py-2 rounded-lg border text-sm focus:border-accent outline-none font-mono"
                  value={editing.url || ''} onChange={(e) => setEditing({ ...editing, url: e.target.value })} required
                />
              </div>
              <div className="grid grid-cols-2 gap-3">
                <div>
                  <label className="text-xs text-fg-3 block mb-1">请求方式</label>
                  <select
                    className="w-full bg-bg px-3 py-2 rounded-lg border text-sm focus:border-accent outline-none"
                    value={editing.method || 'GET'} onChange={(e) => setEditing({ ...editing, method: e.target.value })}
                  >
                    <option>GET</option><option>HEAD</option>
                  </select>
                </div>
                <div>
                  <label className="text-xs text-fg-3 block mb-1">预期状态码</label>
                  <input
                    className="w-full bg-bg px-3 py-2 rounded-lg border text-sm focus:border-accent outline-none"
                    type="number" value={editing.expected_status || 200} onChange={(e) => setEditing({ ...editing, expected_status: Number(e.target.value) })}
                  />
                </div>
              </div>
              <div>
                <label className="text-xs text-fg-3 block mb-1">检测间隔（秒）</label>
                <input
                  className="w-full bg-bg px-3 py-2 rounded-lg border text-sm focus:border-accent outline-none"
                  type="number" value={editing.interval_s || 60} onChange={(e) => setEditing({ ...editing, interval_s: Number(e.target.value) })}
                />
              </div>
              <div className="flex items-center justify-between">
                <label className="text-xs text-fg-3">启用监控</label>
                <button
                  type="button"
                  onClick={() => setEditing({ ...editing, enabled: !editing.enabled })}
                  className={`relative w-10 h-5 rounded-full transition-colors ${editing.enabled ? 'bg-accent' : 'bg-gray-600'}`}
                >
                  <span className={`absolute top-0.5 left-0.5 w-4 h-4 rounded-full bg-white transition-transform ${editing.enabled ? 'translate-x-5' : ''}`} />
                </button>
              </div>
              <div className="flex gap-3 pt-2">
                <button type="submit" className="flex-1 bg-accent text-black rounded-lg py-2 text-sm font-medium">保存</button>
                <button type="button" onClick={() => setEditing(null)} className="flex-1 bg-bg-hover rounded-lg py-2 text-sm">取消</button>
              </div>
            </form>
          </div>
        </div>
      )}
    </div>
  )
}
