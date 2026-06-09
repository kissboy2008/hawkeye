import { useQuery } from '@tanstack/react-query'
import { useMemo, memo, useState } from 'react'
import { agents, metrics, widgets, type Widget } from '../api/client'
import type { Agent } from '../types'
import { ProgressBar, getBarColor } from '../components/ProgressBar'
import { renderWidget } from '../components/WidgetCards'

function formatUptime(seconds: number): string {
 if (seconds <= 0) return ''
 const d = Math.floor(seconds / 86400)
 const h = Math.floor((seconds % 86400) / 3600)
 const m = Math.floor((seconds % 3600) / 60)
 if (d > 0) return `${d}天 ${h}小时`
 if (h > 0) return `${h}小时 ${m}分钟`
 return `${m}分钟`
}

function StatusDot({ status }: { status: string }) {
 const colors: Record<string, string> = {
  online: 'bg-ok',
  offline: 'bg-err',
  unknown: 'bg-gray-500',
 }
 return <span className={`inline-block w-2.5 h-2.5 rounded-full ${colors[status] || colors.unknown}`} />
}

const AgentCard = memo(function AgentCard({ agent }: { agent: Agent }) {
 const [ctxMenu, setCtxMenu] = useState<{ x: number; y: number } | null>(null)

 const { data: latest } = useQuery({
  queryKey: ['latest', agent.id],
  queryFn: () => metrics.latest(agent.id),
  enabled: agent.status === 'online',
  refetchInterval: 15000,
 })

 const cpu = latest?.metrics?.cpu ? JSON.parse(latest.metrics.cpu) : null
 const mem = latest?.metrics?.memory ? JSON.parse(latest.metrics.memory) : null
 const uptimeData = latest?.metrics?.uptime ? JSON.parse(latest.metrics.uptime) : null
 const uptimeS = uptimeData?.uptime_seconds || 0
 const hostname = agent.address

 const intranetUrl = agent.intranet_url || ''
 const extranetUrl = agent.extranet_url || ''

 const openExternal = () => {
  if (extranetUrl) {
   window.open(extranetUrl, '_blank', 'noopener,noreferrer')
  }
  setCtxMenu(null)
 }

 if (agent.status !== 'online') {
  return (
   <div className="bg-bg-card/60 rounded-xl p-3 md:p-4 opacity-60">
    <div className="flex items-center gap-2 mb-3">
     <StatusDot status={agent.status} />
     <span className="font-medium text-sm md:text-base">{agent.name}</span>
    </div>
    <div className="text-xs text-white truncate">{agent.address}</div>
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

 return (
  <>
   <div
    className={`bg-bg-card/70 rounded-xl p-3 md:p-4 hover:border-purple-400/30 transition-colors ${intranetUrl ? 'cursor-pointer' : ''}`}
    onClick={() => { if (intranetUrl) window.open(intranetUrl, '_blank', 'noopener,noreferrer') }}
    onContextMenu={e => { e.preventDefault(); setCtxMenu({ x: e.clientX, y: e.clientY }) }}
   >
    <div className="flex items-center justify-between mb-3">
     <div className="flex items-center gap-2 min-w-0">
      <StatusDot status={agent.status} />
      <span className="font-medium text-sm md:text-base truncate">{agent.name}</span>
     </div>
    </div>

    <div className="space-y-2 mb-3">
     <div className="flex items-center gap-3">
      <span className="text-xs text-white w-9 shrink-0">Uptime</span>
      <span className="text-xs text-white flex-1 text-right tabular-nums">{formatUptime(uptimeS) || '--'}</span>
     </div>
     <div className="flex items-center gap-3">
      <span className="text-xs text-white w-9 shrink-0">Load</span>
      <div className="flex items-center gap-1.5 flex-1 justify-end">
       <span title="1 min" className={`text-xs tabular-nums ${load1 > cores ? 'text-err' : load1 > cores * 0.7 ? 'text-warn' : 'text-white'}`}>
        {load1.toFixed(2)}
       </span>
       <span title="5 min" className={`text-xs tabular-nums ${load5 > cores ? 'text-err' : load5 > cores * 0.7 ? 'text-warn' : 'text-white'}`}>
        {load5.toFixed(2)}
       </span>
       <span title="15 min" className={`text-xs tabular-nums ${load15 > cores ? 'text-err' : load15 > cores * 0.7 ? 'text-warn' : 'text-white'}`}>
        {load15.toFixed(2)}
       </span>
      </div>
     </div>
     <div className="flex items-center gap-3">
      <span className="text-xs text-white w-9 shrink-0">CPU</span>
      <div className="flex-1"><ProgressBar value={cpuPct} colorClass={getBarColor(cpuPct, 50, 80)} /></div>
      <span className="text-xs text-white w-11 text-right tabular-nums">{cpuPct.toFixed(1)}%</span>
     </div>
     <div className="flex items-center gap-3">
      <span className="text-xs text-white w-9 shrink-0">MEM</span>
      <div className="flex-1"><ProgressBar value={memPct} colorClass={getBarColor(memPct, 70, 85)} /></div>
      <span className="text-xs text-white w-11 text-right tabular-nums">{memPct.toFixed(1)}%</span>
     </div>
    </div>

    <div className="flex items-center justify-between gap-2 text-xs text-white">
     <span className="truncate">{cpu?.model_name || hostname}</span>
     {cpu?.kernel_version ? <span className="shrink-0">{cpu.kernel_version}</span> : null}
    </div>
   </div>

   {ctxMenu && (
    <>
     <div className="fixed inset-0 z-50" onClick={() => setCtxMenu(null)} onContextMenu={e => { e.preventDefault(); setCtxMenu(null) }} />
     <div
      className="fixed z-50 bg-bg-card/70 rounded-lg shadow-glow py-1 min-w-[120px] animate-in fade-in zoom-in-95 duration-100"
      style={{ left: ctxMenu.x, top: ctxMenu.y }}
     >
      <button
       onClick={openExternal}
       className="w-full px-3 py-2 text-left text-sm text-white hover:bg-white/5 transition-colors flex items-center gap-2"
      >
       <svg className="w-3.5 h-3.5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
        <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M10 6H6a2 2 0 00-2 2v10a2 2 0 002 2h10a2 2 0 002-2v-4M14 4h6m0 0v6m0-6L10 14" />
       </svg>
       跳转外网
      </button>
     </div>
    </>
   )}
  </>
 )
})

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

 const colCount = Math.min(groups.length || 1, 5)
 const gridColsClass = colCount <= 1
  ? ''
  : colCount === 2
  ? 'lg:grid-cols-2'
  : colCount === 3
  ? 'lg:grid-cols-3'
  : colCount === 4
  ? 'lg:grid-cols-4'
  : 'lg:grid-cols-5'

 return (
  <div className="py-4 px-2 md:py-6 md:px-3 max-w-[1920px] mx-auto">
   {/* Host Status */}

   <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 xl:grid-cols-5 gap-4 mb-8">
    {agentList.map((agent) => (
     <AgentCard key={agent.id} agent={agent} />
    ))}
    {agentList.length === 0 && (
     <div className="col-span-full text-center py-16 text-white">
      暂无监控机器，请在「机器管理」页面添加
     </div>
    )}
   </div>

   {/* 服务组件 */}
   {widgetList.length > 0 && (
    <>

     <div className={`grid grid-cols-1 md:grid-cols-2 ${gridColsClass} gap-6 mb-8`}>
      {groups.map(([groupName, groupWidgets]) => {
       if (groupWidgets.length === 0) return null
       return (
        <div key={groupName} className="flex flex-col gap-3">
         <div className="flex items-center pb-2">
          <h4 className="text-sm font-bold text-white uppercase tracking-wider px-1">
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
