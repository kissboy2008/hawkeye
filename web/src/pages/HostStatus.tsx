import { useState } from 'react'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { agents, metrics, alertEvents } from '../api/client'
import type { Agent } from '../types'
import { useNavigate } from 'react-router-dom'
import { toCSTFull } from '../utils'
import { DragHandle } from '../components/probe/DragHandle'
import { ProgressBar, getBarColor } from '../components/ProgressBar'
import {
 DndContext,
 closestCenter,
 KeyboardSensor,
 PointerSensor,
 useSensor,
 useSensors,
 type DragEndEvent,
} from '@dnd-kit/core'
import {
 SortableContext,
 sortableKeyboardCoordinates,
 verticalListSortingStrategy,
 useSortable,
} from '@dnd-kit/sortable'
import { CSS } from '@dnd-kit/utilities'



interface SortableHostCardProps {
 agent: Agent
}

function SortableHostCard({ agent }: SortableHostCardProps) {
 const navigate = useNavigate()
 const [expanded, setExpanded] = useState(false)
 const {
  attributes,
  listeners,
  setNodeRef,
  transform,
  transition,
  isDragging,
 } = useSortable({ id: agent.id })

 const style = {
  transform: CSS.Transform.toString(transform),
  transition,
  opacity: isDragging ? 0.5 : 1,
  zIndex: isDragging ? 10 : undefined,
 }

    const { data: latest } = useQuery({
      queryKey: ['latest', agent.id],
      queryFn: () => metrics.latest(agent.id),
      enabled: agent.status === 'online',
    })
 const cpu = latest?.metrics?.cpu ? JSON.parse(latest.metrics.cpu) : null
 const mem = latest?.metrics?.memory ? JSON.parse(latest.metrics.memory) : null
 const hostname = agent.address

 const cpuPct = cpu?.usage_percent ?? 0
 const memPct = mem?.usage_percent ?? 0
 const memUsedGb = mem ? (mem.used_mb / 1024).toFixed(1) : '-'
 const memTotalGb = mem ? (mem.total_mb / 1024).toFixed(1) : '-'
 const load1 = cpu?.load1 ?? 0
 const load5 = cpu?.load5 ?? 0
 const load15 = cpu?.load15 ?? 0
 const cores = cpu?.cores ?? 1
 const hasLoad = load1 > 0 || load5 > 0 || load15 > 0
 const modelName = cpu?.model_name || ''
 const lastSeen = agent.last_seen ? new Date(agent.last_seen) : null
 const timeSince = lastSeen ? formatTimeSince(lastSeen) : ''

 return (
  <div
   ref={setNodeRef}
   style={style}
   {...attributes}
   className={`group bg-bg-card/70 rounded-xl overflow-hidden shadow-soft hover:shadow-glow transition-shadow ${isDragging ? 'shadow-glow' : ''}`}
  >
   {/* Main row */}
   <div
    className="flex items-center gap-3 px-4 py-4 cursor-pointer hover:bg-bg-hover/30 transition-colors"
    onClick={() => setExpanded(!expanded)}
   >
    <DragHandle listeners={listeners} />
    {/* Status dot */}
    <span className={`w-2.5 h-2.5 rounded-full flex-shrink-0 ${
     agent.status === 'online' ? 'bg-ok' : agent.status === 'offline' ? 'bg-err' : 'bg-gray-500'
    }`} />

    {/* Name & info */}
    <div className="flex-1 min-w-0">
     <div className="flex items-center gap-2">
      <span className="font-medium text-sm">{agent.name}</span>
      {agent.agent_version && (
       <span className="text-[10px] text-gray-500 font-mono bg-bg-hover px-1.5 py-0.5 rounded">{agent.agent_version}</span>
      )}
      {/* Work mode indicator */}
      <span
       className={`text-[10px] font-mono px-1.5 py-0.5 rounded ${
        agent.mode === 'pull'
         ? 'bg-blue-100 text-blue-600 dark:bg-blue-900/30 dark:text-blue-400'
         : 'bg-green-100 text-green-600 dark:bg-green-900/30 dark:text-green-400'
       }`}
       title={agent.mode === 'pull' ? '服务端主动拉取 (Pull)' : 'Agent主动推送 (Push)'}
      >
       {agent.mode === 'pull' ? '↓Pull' : '↑Push'}
      </span>
     </div>
     <span className="text-xs text-gray-500 font-mono truncate block">{hostname}</span>
    </div>

    {/* Quick stats for online agents */}
    {agent.status === 'online' && cpu && mem && (
     <div className="flex items-center gap-4 text-xs flex-shrink-0">
      <div className="text-center">
       <div className={`font-semibold ${cpuPct >= 80 ? 'text-err' : cpuPct >= 50 ? 'text-warn' : 'text-gray-300'}`}>
        {cpuPct.toFixed(1)}%
       </div>
       <div className="text-gray-500">CPU</div>
      </div>
      <div className="text-center">
       <div className={`font-semibold ${memPct >= 85 ? 'text-err' : memPct >= 70 ? 'text-warn' : 'text-gray-300'}`}>
        {memPct.toFixed(1)}%
       </div>
       <div className="text-gray-500">MEM</div>
      </div>
     </div>
    )}

    {/* Offline time */}
    {agent.status !== 'online' && (
     <div className="text-xs text-gray-500 flex-shrink-0">
      {timeSince ? `离线 ${timeSince}` : '离线'}
     </div>
    )}

    {/* Expand arrow */}
    <svg
     className={`w-4 h-4 text-gray-500 flex-shrink-0 transition-transform duration-200 ${expanded ? 'rotate-180' : ''}`}
     fill="none" stroke="currentColor" viewBox="0 0 24 24"
    >
     <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M19 9l-7 7-7-7" />
    </svg>
   </div>

   {/* Expanded detail */}
   {expanded && (
    <div className="px-4 py-3">
     {agent.status === 'online' && cpu && mem ? (
      <div className="space-y-3">
       {/* Load */}
       {hasLoad && (
        <div className="flex items-center gap-3">
         <span className="text-xs text-gray-400 w-12 shrink-0">LOAD</span>
         <div className="flex items-center gap-2 flex-1 justify-end">
          <span title="1 min" className={`text-xs tabular-nums font-mono ${load1 > cores ? 'text-err' : load1 > cores * 0.7 ? 'text-warn' : 'text-gray-300'}`}>
           {load1.toFixed(2)}
          </span>
          <span title="5 min" className={`text-xs tabular-nums font-mono ${load5 > cores ? 'text-err' : load5 > cores * 0.7 ? 'text-warn' : 'text-gray-300'}`}>
           {load5.toFixed(2)}
          </span>
          <span title="15 min" className={`text-xs tabular-nums font-mono ${load15 > cores ? 'text-err' : load15 > cores * 0.7 ? 'text-warn' : 'text-gray-300'}`}>
           {load15.toFixed(2)}
          </span>
         </div>
        </div>
       )}

       {/* CPU */}
       <div className="flex items-center gap-3">
        <span className="text-xs text-gray-400 w-12 shrink-0">CPU</span>
        <div className="flex-1"><ProgressBar value={cpuPct} colorClass={getBarColor(cpuPct, 50, 80)} /></div>
        <span className="text-xs text-gray-300 w-14 text-right tabular-nums">{cpuPct.toFixed(1)}% ({cores}核)</span>
       </div>

       {/* Memory */}
       <div className="flex items-center gap-3">
        <span className="text-xs text-gray-400 w-12 shrink-0">MEM</span>
        <div className="flex-1"><ProgressBar value={memPct} colorClass={getBarColor(memPct, 70, 85)} /></div>
        <span className="text-xs text-gray-300 w-14 text-right tabular-nums">{memUsedGb}/{memTotalGb} GB</span>
       </div>

       {/* CPU model */}
       {modelName && (
        <div className="text-xs text-gray-500 truncate pt-1">{modelName}</div>
       )}

       {/* Last seen */}
       {lastSeen && (
        <div className="text-xs text-gray-500">
         最后上报: {agent.last_seen ? toCSTFull(agent.last_seen) : ''}
        </div>
       )}
      </div>
     ) : (
      <div className="py-4 text-center text-gray-500 text-sm">
       {agent.status === 'offline' ? '主机离线，暂无数据' : '等待数据上报...'}
      </div>
     )}

     {/* Action buttons */}
     <div className="flex items-center justify-end gap-2 pt-3">
      <button
       onClick={(e) => { e.stopPropagation(); navigate(`/machine/${encodeURIComponent(agent.name)}`) }}
       className="px-3 py-1 rounded-lg text-xs gradient-bar text-white font-medium hover:opacity-80 transition-opacity"
      >
       详情
      </button>
     </div>
    </div>
   )}
  </div>
 )
}

function formatTimeSince(date: Date): string {
 const seconds = Math.floor((Date.now() - date.getTime()) / 1000)
 if (seconds < 60) return '刚刚'
 const minutes = Math.floor(seconds / 60)
 if (minutes < 60) return `${minutes}分钟`
 const hours = Math.floor(minutes / 60)
 if (hours < 24) return `${hours}小时`
 const days = Math.floor(hours / 24)
 return `${days}天`
}

function GridHostCard({ agent }: SortableHostCardProps) {
 const navigate = useNavigate()

    const { data: latest } = useQuery({
      queryKey: ['latest', agent.id],
      queryFn: () => metrics.latest(agent.id),
      enabled: agent.status === 'online',
    })
 const cpu = latest?.metrics?.cpu ? JSON.parse(latest.metrics.cpu) : null
 const mem = latest?.metrics?.memory ? JSON.parse(latest.metrics.memory) : null
 const hostname = agent.address

 const cpuPct = cpu?.usage_percent ?? 0
 const memPct = mem?.usage_percent ?? 0
 const memUsedGb = mem ? (mem.used_mb / 1024).toFixed(1) : '-'
 const memTotalGb = mem ? (mem.total_mb / 1024).toFixed(1) : '-'
 const cores = cpu?.cores ?? 1
 const lastSeen = agent.last_seen ? new Date(agent.last_seen) : null
 const timeSince = lastSeen ? formatTimeSince(lastSeen) : ''

 return (
  <div
   className="bg-bg-card/70 rounded-xl p-3 flex flex-col gap-3 shadow-soft hover:shadow-glow hover:border-accent/20 transition-all cursor-pointer"
   onClick={() => navigate(`/machine/${encodeURIComponent(agent.name)}`)}
  >
   {/* Header */}
   <div className="flex items-center gap-2">
    <span className={`w-2.5 h-2.5 rounded-full flex-shrink-0 ${
     agent.status === 'online' ? 'bg-ok' : agent.status === 'offline' ? 'bg-err' : 'bg-gray-500'
    }`} />
    <span className="font-medium text-sm truncate flex-1">{agent.name}</span>
    <span
     className={`text-[10px] font-mono px-1.5 py-0.5 rounded ${
      agent.mode === 'pull'
       ? 'bg-blue-100 text-blue-600 dark:bg-blue-900/30 dark:text-blue-400'
       : 'bg-green-100 text-green-600 dark:bg-green-900/30 dark:text-green-400'
     }`}
    >
     {agent.mode === 'pull' ? '↓Pull' : '↑Push'}
    </span>
   </div>

   {/* Host address */}
   <div className="text-xs text-gray-500 font-mono truncate">{hostname}</div>

   {/* Metrics */}
   {agent.status === 'online' && cpu && mem ? (
    <div className="space-y-2 flex-1">
     <div>
      <div className="flex justify-between text-xs mb-1">
       <span className="text-gray-400">CPU ({cores}核)</span>
       <span className={`tabular-nums ${cpuPct >= 80 ? 'text-err' : cpuPct >= 50 ? 'text-warn' : 'text-gray-300'}`}>{cpuPct.toFixed(1)}%</span>
      </div>
      <ProgressBar value={cpuPct} colorClass={getBarColor(cpuPct, 50, 80)} />
     </div>
     <div>
      <div className="flex justify-between text-xs mb-1">
       <span className="text-gray-400">MEM</span>
       <span className={`tabular-nums ${memPct >= 85 ? 'text-err' : memPct >= 70 ? 'text-warn' : 'text-gray-300'}`}>{memUsedGb}/{memTotalGb}G</span>
      </div>
      <ProgressBar value={memPct} colorClass={getBarColor(memPct, 70, 85)} />
     </div>
    </div>
   ) : (
    <div className="flex-1 flex items-center justify-center text-xs text-gray-500 py-4">
     {agent.status === 'offline' ? (
      <span>离线 {timeSince}</span>
     ) : (
      <span>等待数据...</span>
     )}
    </div>
   )}

   {/* Version */}
   {agent.agent_version && (
    <div className="text-[10px] text-gray-500 font-mono">{agent.agent_version}</div>
   )}
  </div>
 )
}

export default function HostStatus() {
 const [viewMode, setViewMode] = useState<'list' | 'grid'>('list')
 const queryClient = useQueryClient()
 const { data: agentList = [] } = useQuery({
  queryKey: ['agents'],
  queryFn: agents.list,
  refetchInterval: 30000,
 })

 const agentReorder = useMutation({
  mutationFn: (ids: number[]) => agents.reorder(ids),
  onSuccess: () => queryClient.invalidateQueries({ queryKey: ['agents'] }),
 })

 const { data: activeAlerts = [] } = useQuery({
  queryKey: ['active-alerts'],
  queryFn: () => alertEvents.list(false, 100),
  refetchInterval: 30000,
 })

 const onlineCount = agentList.filter((a) => a.status === 'online').length

 const sensors = useSensors(
  useSensor(PointerSensor, { activationConstraint: { distance: 5 } }),
  useSensor(KeyboardSensor, { coordinateGetter: sortableKeyboardCoordinates }),
 )

 function handleDragEnd(event: DragEndEvent) {
  const { active, over } = event
  if (!over || active.id === over.id) return
  const oldIndex = agentList.findIndex((a) => a.id === active.id)
  const newIndex = agentList.findIndex((a) => a.id === over.id)
  if (oldIndex === -1 || newIndex === -1) return
  const newList = [...agentList]
  const [moved] = newList.splice(oldIndex, 1)
  newList.splice(newIndex, 0, moved)
  agentReorder.mutate(newList.map((a) => a.id))
 }

 return (
  <div className="py-4 px-2 md:py-6 md:px-3 max-w-[1920px] mx-auto">
   {/* Header */}
   <div className="flex items-center justify-between mb-4 md:mb-6">
    <div>
     <h2 className="text-xl md:text-2xl font-bold mb-1">主机状态</h2>
     <p className="text-gray-500 text-sm">服务器 & 设备运行状态</p>
    </div>
    {/* View toggle */}
    <div className="flex items-center gap-1 bg-bg-hover rounded-lg p-1">
     <button
      onClick={() => setViewMode('list')}
      className={`p-1.5 rounded-md transition-colors ${viewMode === 'list' ? 'bg-bg-card/70 text-accent shadow-sm' : 'text-gray-500 hover:text-gray-300'}`}
      title="列表视图"
      aria-label="列表视图"
     >
      <svg className="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
       <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M4 6h16M4 12h16M4 18h16" />
      </svg>
     </button>
     <button
      onClick={() => setViewMode('grid')}
      className={`p-1.5 rounded-md transition-colors ${viewMode === 'grid' ? 'bg-bg-card/70 text-accent shadow-sm' : 'text-gray-500 hover:text-gray-300'}`}
      title="卡片视图"
      aria-label="卡片视图"
     >
      <svg className="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
       <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M4 5a1 1 0 011-1h4a1 1 0 011 1v4a1 1 0 01-1 1H5a1 1 0 01-1-1V5zM14 5a1 1 0 011-1h4a1 1 0 011 1v4a1 1 0 01-1 1h-4a1 1 0 01-1-1V5zM4 15a1 1 0 011-1h4a1 1 0 011 1v4a1 1 0 01-1 1H5a1 1 0 01-1-1v-4zM14 15a1 1 0 011-1h4a1 1 0 011 1v4a1 1 0 01-1 1h-4a1 1 0 01-1-1v-4z" />
      </svg>
     </button>
    </div>
   </div>

   {/* Summary cards */}
   <div className="grid grid-cols-2 sm:grid-cols-3 gap-3 mb-6">
    <div className="bg-bg-card/70 rounded-xl p-3 shadow-soft flex flex-col items-center justify-center">
     <div className="text-xs text-gray-500 mb-1">总主机数</div>
     <div className="text-xl font-bold gradient-text">{agentList.length}</div>
    </div>
    <div className="bg-bg-card/70 rounded-xl p-3 shadow-soft flex flex-col items-center justify-center">
     <div className="text-xs text-gray-500 mb-1">在线</div>
     <div className="text-xl font-bold text-ok">{onlineCount}</div>
    </div>
    <div className="bg-bg-card/70 rounded-xl p-3 shadow-soft flex flex-col items-center justify-center">
     <div className="text-xs text-gray-500 mb-1">活跃告警</div>
     <div className={`text-xl font-bold ${activeAlerts.length > 0 ? 'text-err' : 'text-ok'}`}>
      {activeAlerts.length}
     </div>
    </div>
   </div>

   {/* Host list */}
   {viewMode === 'list' ? (
    <DndContext sensors={sensors} collisionDetection={closestCenter} onDragEnd={handleDragEnd}>
     <SortableContext items={agentList.map((a) => a.id)} strategy={verticalListSortingStrategy}>
      <div className="space-y-3">
       {agentList.map((agent) => (
        <SortableHostCard key={agent.id} agent={agent} />
       ))}

       {agentList.length === 0 && (
        <div className="py-16 text-center text-gray-500">
         <div className="text-4xl mb-3">🖥️</div>
         <div>暂无监控主机</div>
        </div>
       )}
      </div>
     </SortableContext>
    </DndContext>
   ) : (
    <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 xl:grid-cols-4 gap-4">
     {agentList.map((agent) => (
      <GridHostCard key={agent.id} agent={agent} />
     ))}

     {agentList.length === 0 && (
      <div className="col-span-full py-16 text-center text-gray-500">
       <div className="text-4xl mb-3">🖥️</div>
       <div>暂无监控主机</div>
      </div>
     )}
    </div>
   )}
  </div>
 )
}
