import { useQuery } from '@tanstack/react-query'
import { agents, alertEvents } from '../api/client'
import type { Agent } from '../types'
import { useNavigate } from 'react-router-dom'
import { formatTimeSince } from '../utils'
import { useAgentMetrics } from '../hooks/useAgentMetrics'
import { ProgressBar, getBarColor } from '../components/ProgressBar'

interface HostCardProps {
 agent: Agent
}

function GridHostCard({ agent }: HostCardProps) {
 const navigate = useNavigate()

    const { cpu, mem, hostname, cpuPct, memPct, memUsedGb, memTotalGb, cores } = useAgentMetrics(agent)
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
       ? 'bg-blue-500/15 text-blue-300'
       : 'bg-green-500/15 text-green-300'
     }`}
    >
     {agent.mode === 'pull' ? '↓Pull' : '↑Push'}
    </span>
   </div>

   {/* Host address */}
   <div className="text-xs text-fg-3 font-mono truncate">{hostname}</div>

   {/* Metrics */}
   {agent.status === 'online' && cpu && mem ? (
    <div className="space-y-2 flex-1">
     <div>
      <div className="flex justify-between text-xs mb-1">
       <span className="text-fg-3">CPU ({cores}核)</span>
       <span className={`tabular-nums ${cpuPct >= 80 ? 'text-err' : cpuPct >= 50 ? 'text-warn' : 'text-fg-2'}`}>{cpuPct.toFixed(1)}%</span>
      </div>
      <ProgressBar value={cpuPct} colorClass={getBarColor(cpuPct, 50, 80)} />
     </div>
     <div>
      <div className="flex justify-between text-xs mb-1">
       <span className="text-fg-3">MEM</span>
       <span className={`tabular-nums ${memPct >= 85 ? 'text-err' : memPct >= 70 ? 'text-warn' : 'text-fg-2'}`}>{memUsedGb}/{memTotalGb}G</span>
      </div>
      <ProgressBar value={memPct} colorClass={getBarColor(memPct, 70, 85)} />
     </div>
    </div>
   ) : (
    <div className="flex-1 flex items-center justify-center text-xs text-fg-3 py-4">
     {agent.status === 'offline' ? (
      <span>离线 {timeSince}</span>
     ) : (
      <span>等待数据...</span>
     )}
    </div>
   )}

   {/* Version */}
   {agent.agent_version && (
    <div className="text-[10px] text-fg-3 font-mono">{agent.agent_version}</div>
   )}
  </div>
 )
}

export default function HostStatus() {
 const { data: agentList = [] } = useQuery({
  queryKey: ['agents'],
  queryFn: agents.list,
  refetchInterval: 30000,
 })

 const { data: activeAlerts = [] } = useQuery({
  queryKey: ['active-alerts'],
  queryFn: () => alertEvents.list(false, 100),
  refetchInterval: 30000,
 })

 const onlineCount = agentList.filter((a) => a.status === 'online').length

 return (
  <div className="py-4 px-2 md:py-6 md:px-3 max-w-[1920px] mx-auto">
   {/* Header */}
   <div className="mb-4 md:mb-6">
    <h2 className="page-title">主机状态</h2>
    <p className="text-fg-3 text-sm">服务器 & 设备运行状态</p>
   </div>

   {/* Summary cards */}
   <div className="grid grid-cols-2 sm:grid-cols-3 gap-3 mb-6">
    <div className="bg-bg-card/70 rounded-xl p-3 shadow-soft flex flex-col items-center justify-center">
     <div className="text-xs text-fg-3 mb-1">总主机数</div>
     <div className="text-xl font-bold gradient-text">{agentList.length}</div>
    </div>
    <div className="bg-bg-card/70 rounded-xl p-3 shadow-soft flex flex-col items-center justify-center">
     <div className="text-xs text-fg-3 mb-1">在线</div>
     <div className="text-xl font-bold text-ok">{onlineCount}</div>
    </div>
    <div className="bg-bg-card/70 rounded-xl p-3 shadow-soft flex flex-col items-center justify-center">
     <div className="text-xs text-fg-3 mb-1">活跃告警</div>
     <div className={`text-xl font-bold ${activeAlerts.length > 0 ? 'text-err' : 'text-ok'}`}>
      {activeAlerts.length}
     </div>
    </div>
   </div>

   {/* Host cards */}
   <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 xl:grid-cols-4 gap-4">
    {agentList.map((agent) => (
     <GridHostCard key={agent.id} agent={agent} />
    ))}

    {agentList.length === 0 && (
     <div className="col-span-full py-16 text-center text-fg-3">
      <div className="text-4xl mb-3">🖥️</div>
      <div>暂无监控主机</div>
     </div>
    )}
   </div>
  </div>
 )
}
