import { useQuery } from '@tanstack/react-query'
import { useMemo } from 'react'
import { agents, metrics, widgets, type Widget } from '../api/client'
import type { Agent } from '../types'
import { useNavigate } from 'react-router-dom'
import { ProgressBar, getBarColor } from '../components/ProgressBar'
import { renderWidget } from '../components/WidgetCards'

function StatusDot({ status }: { status: string }) {
  const colors: Record<string, string> = {
    online: 'bg-ok',
    offline: 'bg-err',
    unknown: 'bg-gray-500',
  }
  return <span className={`inline-block w-2.5 h-2.5 rounded-full ${colors[status] || colors.unknown}`} />
}

function AgentCard({ agent }: { agent: Agent }) {
  const navigate = useNavigate()

  const { data: latest } = useQuery({
    queryKey: ['latest', agent.id],
    queryFn: () => metrics.latest(agent.id),
    enabled: agent.status === 'online',
    refetchInterval: 15000,
  })

  const cpu = latest?.metrics?.cpu ? JSON.parse(latest.metrics.cpu) : null
  const mem = latest?.metrics?.memory ? JSON.parse(latest.metrics.memory) : null
  const hostname = agent.address

  if (agent.status !== 'online') {
    return (
      <div className="bg-bg-card/60 rounded-xl p-3 md:p-4 border border-purple-500/10 opacity-60">
        <div className="flex items-center gap-2 mb-3">
          <StatusDot status={agent.status} />
          <span className="font-medium text-sm md:text-base">{agent.name}</span>
        </div>
        <div className="text-xs text-gray-500 truncate">{agent.address}</div>
        <div className="text-sm text-err mt-1">离线</div>
      </div>
    )
  }

  const cpuPct = cpu?.usage_percent ?? 0
  const memPct = mem?.usage_percent ?? 0
  const load1 = cpu?.load1 ?? 0
  const load5 = cpu?.load5 ?? 0
  const load15 = cpu?.load15 ?? 0
  const cores = cpu?.cores ?? 1
  const hasLoad = load1 > 0 || load5 > 0 || load15 > 0

  return (
    <div
      className="bg-bg-card rounded-xl p-3 md:p-4 border border-purple-500/10 hover:border-purple-400/30 transition-colors cursor-pointer"
      onClick={() => navigate(`/machine/${encodeURIComponent(agent.name)}`)}
    >
      <div className="flex items-center justify-between mb-3">
        <div className="flex items-center gap-2 min-w-0">
          <StatusDot status={agent.status} />
          <span className="font-medium text-sm md:text-base truncate">{agent.name}</span>
        </div>
      </div>

      <div className="text-xs text-gray-500 truncate mb-3">{hostname}</div>

      <div className="space-y-2 mb-3">
        {hasLoad && (
          <div className="flex items-center gap-3">
            <span className="text-xs text-gray-400 w-9 shrink-0">LOAD</span>
            <div className="flex items-center gap-1.5 flex-1 justify-end">
              <span title="1 min" className={`text-xs tabular-nums ${load1 > cores ? 'text-err' : load1 > cores * 0.7 ? 'text-warn' : 'text-gray-300'}`}>
                {load1.toFixed(2)}
              </span>
              <span title="5 min" className={`text-xs tabular-nums ${load5 > cores ? 'text-err' : load5 > cores * 0.7 ? 'text-warn' : 'text-gray-300'}`}>
                {load5.toFixed(2)}
              </span>
              <span title="15 min" className={`text-xs tabular-nums ${load15 > cores ? 'text-err' : load15 > cores * 0.7 ? 'text-warn' : 'text-gray-300'}`}>
                {load15.toFixed(2)}
              </span>
            </div>
          </div>
        )}
        <div className="flex items-center gap-3">
          <span className="text-xs text-gray-400 w-9 shrink-0">CPU</span>
          <div className="flex-1"><ProgressBar value={cpuPct} colorClass={getBarColor(cpuPct, 50, 80)} /></div>
          <span className="text-xs text-gray-300 w-11 text-right tabular-nums">{cpuPct.toFixed(1)}%</span>
        </div>
        <div className="flex items-center gap-3">
          <span className="text-xs text-gray-400 w-9 shrink-0">MEM</span>
          <div className="flex-1"><ProgressBar value={memPct} colorClass={getBarColor(memPct, 70, 85)} /></div>
          <span className="text-xs text-gray-300 w-11 text-right tabular-nums">{memPct.toFixed(1)}%</span>
        </div>
      </div>

      <div className="flex items-center justify-between gap-2 text-xs text-gray-500">
        <span className="truncate">{cpu?.model_name || hostname}</span>
        {cpu?.kernel_version ? <span className="shrink-0">{cpu.kernel_version}</span> : null}
      </div>
    </div>
  )
}

export default function Dashboard() {
  const { data: agentList = [] } = useQuery({
    queryKey: ['agents'],
    queryFn: agents.list,
  })

  const { data: widgetList = [] } = useQuery({
    queryKey: ['widgets'],
    queryFn: widgets.list,
    refetchInterval: 30000,
  })

  // Group widgets by widget_group (mirrors HomepageWidgets layout)
  const groups = useMemo(() => {
    const groupMap = new Map<string, Widget[]>()
    for (const w of widgetList) {
      const g = w.widget_group || '未分组'
      if (!groupMap.has(g)) groupMap.set(g, [])
      groupMap.get(g)!.push(w)
    }
    return Array.from(groupMap.entries())
  }, [widgetList])

  const colCount = Math.min(groups.length || 1, 4)
  const gridColsClass = colCount <= 1
    ? ''
    : colCount === 2
    ? 'lg:grid-cols-2'
    : colCount === 3
    ? 'lg:grid-cols-3'
    : 'lg:grid-cols-4'

  return (
    <div className="p-4 md:p-6 max-w-[1800px] mx-auto">
      {/* Host Status */}

      <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 xl:grid-cols-4 gap-4 mb-8">
        {agentList.map((agent) => (
          <AgentCard key={agent.id} agent={agent} />
        ))}
        {agentList.length === 0 && (
          <div className="col-span-full text-center py-16 text-gray-500">
            暂无监控机器，请在「机器管理」页面添加
          </div>
        )}
      </div>

      {/* 服务组件 */}
      {widgetList.length > 0 && (
        <>

          <div className={`grid grid-cols-1 md:grid-cols-2 ${gridColsClass} gap-6 items-start mb-8`}>
            {groups.map(([groupName, groupWidgets]) => {
              if (groupWidgets.length === 0) return null
              return (
                <div key={groupName} className="space-y-3">
                  <div className="flex items-center pb-2 border-b border-purple-500/10">
                    <h4 className="text-sm font-bold text-gray-300 uppercase tracking-wider px-1">
                      {groupName}
                    </h4>
                  </div>
                  <div className="space-y-3">
                    {groupWidgets.map(w => renderWidget(w))}
                  </div>
                </div>
              )
            })}
          </div>
        </>
      )}
    </div>
  )
}
