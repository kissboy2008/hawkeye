import { useState } from 'react'
import { useQuery, useQueryClient } from '@tanstack/react-query'
import { widgets, type Widget } from '../api/client'

// --- Widget Icons ---
const WIDGET_ICONS: Record<string, string> = {
  proxmox: '/icons/proxmox.svg',
  pbs: '/icons/pbs.svg',
  unraid: '/icons/unraid.svg',
  portainer: '/icons/portainer.svg',
  adguard: '/icons/adguard.svg',
  jellyfin: '/icons/jellyfin.svg',
  moviepilot: '/icons/moviepilot.svg',
  openwrt: '/icons/openwrt.svg',
  qbittorrent: '/icons/qbittorrent.svg',
  hawkeye: '/icons/hawkeye.svg',
  lucky: '/icons/lucky.svg',
  transmission: '/icons/transmission.svg',
  homeassistant: '/icons/homeassistant.svg',
  ikuai: '/icons/ikuai.png',
  openclash: '/icons/openclash.svg',
}

function WidgetIcon({ type }: { type: string }) {
  const src = WIDGET_ICONS[type]
  if (!src) return <span className="text-2xl">📦</span>
  return <img src={src} alt={type} className="w-8 h-8 object-contain" />
}

// --- Helper ---
function formatBytes(bytes: number): string {
  if (bytes === 0) return '0 B'
  const k = 1024
  const sizes = ['B', 'KB', 'MB', 'GB', 'TB']
  const i = Math.floor(Math.log(bytes) / Math.log(k))
  return (bytes / Math.pow(k, i)).toFixed(1) + ' ' + sizes[i]
}

function formatSpeed(bytesPerSec: number): string {
  if (bytesPerSec === 0) return '0 B/s'
  const k = 1024
  const sizes = ['B/s', 'KB/s', 'MiB/s', 'GiB/s']
  const i = Math.floor(Math.log(bytesPerSec) / Math.log(k))
  return (bytesPerSec / Math.pow(k, i)).toFixed(1) + ' ' + sizes[i]
}

function formatTicks(ticks: number): string {
  const totalSeconds = Math.floor(ticks / 10000000)
  const hours = Math.floor(totalSeconds / 3600)
  const minutes = Math.floor((totalSeconds % 3600) / 60)
  const seconds = totalSeconds % 60
  if (hours > 0) {
    return `${hours}:${String(minutes).padStart(2, '0')}:${String(seconds).padStart(2, '0')}`
  }
  return `${String(minutes).padStart(2, '0')}:${String(seconds).padStart(2, '0')}`
}

// --- Ping Badge ---
function PingBadge({ pingMs }: { pingMs?: number }) {
  if (pingMs === undefined || pingMs === null) return null
  if (pingMs < 0) {
    return <span className="text-xs text-red-400 font-mono">离线</span>
  }
  const color = pingMs < 50 ? 'text-green-400' : pingMs < 150 ? 'text-yellow-400' : 'text-orange-400'
  return <span className={`text-xs font-mono ${color}`}>{pingMs}ms</span>
}

// --- Block Component ---
function Block({ label, value }: { label: string; value?: string | number }) {
  return (
    <div className="bg-white/5 rounded-md flex-1 flex flex-col items-center justify-center text-center p-2 min-w-0">
      <div className="text-sm text-white font-light tabular-nums truncate w-full">
        {value === undefined || value === null ? '-' : value}
      </div>
      <div className="text-xs font-bold uppercase text-white truncate w-full">{label}</div>
    </div>
  )
}

// --- Widget Title ---
function WidgetTitle({ widget, fallbackDesc, children }: { widget: Widget; fallbackDesc?: string; children?: React.ReactNode }) {
  return (
    <div className="flex items-center gap-3 flex-1 min-w-0">
      {children}
      <div className="min-w-0">
        <div className="font-semibold text-sm truncate">{widget.name}</div>
        <div className="text-xs text-white truncate">{widget.description || fallbackDesc || ''}</div>
      </div>
    </div>
  )
}

function WidgetCard({ widget, children, className = '' }: { widget: Widget; children: React.ReactNode; className?: string }) {
  const config = (() => { try { return JSON.parse(widget.config || '{}') } catch { return {} } })()
  const linkUrl: string = config.link_url || ''
  const externalUrl: string = config.external_url || ''
  const [ctxMenu, setCtxMenu] = useState<{ x: number; y: number } | null>(null)

  const openExternal = () => {
    if (externalUrl) {
      window.open(externalUrl, '_blank', 'noopener,noreferrer')
    }
    setCtxMenu(null)
  }

  return (
    <>
      <div
        className={`bg-bg-card/70 rounded-xl shadow-soft hover:shadow-glow transition-shadow ${linkUrl ? 'cursor-pointer' : ''} ${className}`}
        onClick={() => { if (linkUrl) window.open(linkUrl, '_blank', 'noopener,noreferrer') }}
        onContextMenu={e => { e.preventDefault(); setCtxMenu({ x: e.clientX, y: e.clientY }) }}
      >
        {children}
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
}

// --- Widget Frame (shared query + skeleton + card shell) ---
interface WidgetFrameProps {
  widget: Widget
  fallbackDesc: string | ((data: any) => string)
  iconType: string
  refetchInterval?: number
  retry?: number
  showPing?: boolean
  children: (data: any) => React.ReactNode
}

function WidgetFrame({ widget, fallbackDesc, iconType, refetchInterval = 30000, retry = 1, showPing = true, children }: WidgetFrameProps) {
  const { data, isLoading, error } = useQuery({
    queryKey: ['widget-data', widget.id],
    queryFn: () => widgets.data(widget.id),
    refetchInterval,
    retry,
  })

  if (isLoading) return <WidgetSkeleton name={widget.name} />
  if (error) return <WidgetError name={widget.name} error={(error as Error).message} />

  const desc = typeof fallbackDesc === 'function' ? fallbackDesc(data) : fallbackDesc

  return (
    <WidgetCard widget={widget}>
      <div className="flex items-center gap-3 p-4 pb-2">
        <WidgetTitle widget={widget} fallbackDesc={desc}>
          <WidgetIcon type={iconType} />
        </WidgetTitle>
        {showPing && <div className="ml-auto"><PingBadge pingMs={data.ping_ms} /></div>}
      </div>
      {children(data)}
    </WidgetCard>
  )
}

// --- Proxmox VE Widget ---
function ProxmoxWidget({ widget }: { widget: Widget }) {
  return (
    <WidgetFrame widget={widget} fallbackDesc="Simplify your Data Center" iconType="proxmox">
      {(data) => {
        const cpuPct = ((data.cpu || 0) * 100).toFixed(0) + '%'
        const memPct = data.maxmem > 0 ? ((data.mem / data.maxmem) * 100).toFixed(0) + '%' : '0%'
        const runningVMs = data.vms?.filter((v: any) => v.status === 'running').length || 0
        const totalVMs = data.vms?.length || 0
        const runningCTs = data.cts?.filter((c: any) => c.status === 'running').length || 0
        const totalCTs = data.cts?.length || 0
        return (
          <div className="flex gap-1 px-3 pb-3">
            <Block label="CPU" value={cpuPct} />
            <Block label="内存" value={memPct} />
            <Block label="虚拟机" value={`${runningVMs} / ${totalVMs}`} />
            <Block label="容器" value={`${runningCTs} / ${totalCTs}`} />
          </div>
        )
      }}
    </WidgetFrame>
  )
}

// --- PBS Widget ---
function PBSWidget({ widget }: { widget: Widget }) {
  return (
    <WidgetFrame widget={widget} fallbackDesc="Enterprise Backup Solution" iconType="pbs" refetchInterval={60000}>
      {(data) => {
        const usedPct = (data.used_percent || 0).toFixed(0) + '%'
        const cpuPct = ((data.cpu || 0) * 100).toFixed(0) + '%'
        const memPct = data.maxmem > 0 ? ((data.mem / data.maxmem) * 100).toFixed(0) + '%' : '0%'
        return (
          <div className="flex gap-1 px-3 pb-3">
            <Block label="CPU" value={cpuPct} />
            <Block label="内存" value={memPct} />
            <Block label="数据存储" value={usedPct} />
            <Block label="快照数量" value={data.snapshots || 0} />
          </div>
        )
      }}
    </WidgetFrame>
  )
}

// --- Unraid Widget ---
function UnraidWidget({ widget }: { widget: Widget }) {
  return (
    <WidgetFrame widget={widget} fallbackDesc="网络附加存储" iconType="unraid">
      {(data) => {
        const cpuPct = (data.cpu || 0).toFixed(0) + '%'
        const memPct = (data.mem_percent || 0).toFixed(0) + '%'
        const status = data.array_state === 'STARTED' ? 'Started' : data.array_state
        return (
          <div className="flex gap-1 px-3 pb-3">
            <Block label="CPU" value={cpuPct} />
            <Block label="内存" value={memPct} />
            <Block label="状态" value={status} />
            <Block label="通知" value={data.notif_count ?? 0} />
          </div>
        )
      }}
    </WidgetFrame>
  )
}

// --- Portainer Widget ---
function PortainerWidget({ widget }: { widget: Widget }) {
  return (
    <WidgetFrame widget={widget} fallbackDesc="容器管理" iconType="portainer">
      {(data) => (
        <div className="flex gap-1 px-3 pb-3">
          <Block label="运行中" value={data.running} />
          <Block label="已停止" value={data.stopped} />
          <Block label="总计" value={data.total} />
        </div>
      )}
    </WidgetFrame>
  )
}

// --- AdGuard Home Widget ---
function AdGuardWidget({ widget }: { widget: Widget }) {
  return (
    <WidgetFrame widget={widget} fallbackDesc="DNS 广告过滤" iconType="adguard" refetchInterval={60000}>
      {(data) => {
        const queries = data.queries >= 1000000
          ? (data.queries / 1000000).toFixed(0) + 'M'
          : data.queries >= 1000
          ? (data.queries / 1000).toFixed(0) + 'K'
          : data.queries
        const blocked = data.blocked >= 1000
          ? (data.blocked / 1000).toFixed(0) + 'K'
          : data.blocked
        const avgMs = (data.avg_time * 1000).toFixed(1) + 'ms'
        return (
          <div className="flex gap-1 px-3 pb-3">
            <Block label="查询数" value={queries} />
            <Block label="已拦截" value={blocked} />
            <Block label="过滤" value={data.filtered} />
            <Block label="延迟" value={avgMs} />
          </div>
        )
      }}
    </WidgetFrame>
  )
}

// --- Jellyfin Widget ---
function JellyfinWidget({ widget }: { widget: Widget }) {
  return (
    <WidgetFrame widget={widget} fallbackDesc={(d) => d.status || '媒体服务器'} iconType="jellyfin">
      {(data) => (
        <>
          <div className="flex gap-1 px-3 pb-2">
            <Block label="电影" value={data.movies} />
            <Block label="剧集" value={data.episodes} />
            <Block label="在线用户" value={data.online_users} />
          </div>
          {data.sessions && data.sessions.length > 0 && (
            <div className="px-3 pb-3">
              <div className="text-xs text-white mb-1.5 font-medium">实时会话</div>
              <div className="space-y-1">
                {data.sessions.filter((s: { now_playing?: string }) => s.now_playing).map((s: { user_name: string; now_playing?: string; progress_ticks?: number; runtime_ticks?: number; is_paused?: boolean }, i: number) => (
                  <div key={i} className="bg-white/5 rounded-md px-2.5 py-2 text-xs">
                    <div className="flex items-center justify-between mb-1.5">
                      <span className="text-white font-medium truncate">{s.now_playing} ({s.user_name})</span>
                      {s.runtime_ticks && s.runtime_ticks > 0 && (
                        <span className="text-white flex-shrink-0 ml-2 tabular-nums">
                          {formatTicks(s.progress_ticks || 0)} / {formatTicks(s.runtime_ticks)}
                        </span>
                      )}
                    </div>
                    <div className="flex items-center gap-2">
                      <span className="text-white">{s.is_paused ? '⏸' : '▶'}</span>
                      <div className="flex-1 h-1 bg-white/10 rounded-full overflow-hidden">
                        <div
                          className="h-full bg-purple-400 rounded-full"
                          style={{ width: `${s.runtime_ticks ? Math.min(100, ((s.progress_ticks || 0) / s.runtime_ticks) * 100) : 0}%` }}
                        />
                      </div>
                    </div>
                  </div>
                ))}
              </div>
            </div>
          )}
        </>
      )}
    </WidgetFrame>
  )
}

// --- MoviePilot Widget ---
function MoviePilotWidget({ widget }: { widget: Widget }) {
  return (
    <WidgetFrame widget={widget} fallbackDesc="自动化媒体管理" iconType="moviepilot" refetchInterval={60000}>
      {(data) => (
        <div className="flex gap-1 px-3 pb-3">
          <Block label="电影订阅" value={data.movie_subscribes} />
          <Block label="电视剧订阅" value={data.tv_subscribes} />
          <Block label="总空间" value={data.total_storage} />
          <Block label="剩余空间" value={data.free_storage} />
        </div>
      )}
    </WidgetFrame>
  )
}

// --- Hawkeye Self Status Widget ---
function HawkeyeWidget({ widget }: { widget: Widget }) {
  return (
    <WidgetFrame widget={widget} fallbackDesc="系统监控中心" iconType="hawkeye" refetchInterval={10000}>
      {(data) => (
        <div className="flex gap-1 px-3 pb-3">
          <Block label="在线机器" value={`${data.online_agents}/${data.total_agents}`} />
          <Block label="在线网站" value={`${data.online_probes}/${data.total_probes}`} />
          <Block label="警告" value={data.alerts} />
        </div>
      )}
    </WidgetFrame>
  )
}

// --- Lucky Widget ---
function LuckyWidget({ widget }: { widget: Widget }) {
  return (
    <WidgetFrame widget={widget} fallbackDesc={widget.url} iconType="lucky" refetchInterval={15000}>
      {(data) => (
        <div className="flex gap-1 px-3 pb-3">
          <Block label="CPU" value={data.cpu || '--'} />
          <Block label="反代规则" value={data.rules_count != null ? `${data.enabled_count}/${data.sub_rules_count}` : '--'} />
          <Block label="上传" value={data.net_out_speed || '--'} />
          <Block label="下载" value={data.net_in_speed || '--'} />
        </div>
      )}
    </WidgetFrame>
  )
}

function TransmissionWidget({ widget }: { widget: Widget }) {
  return (
    <WidgetFrame widget={widget} fallbackDesc={widget.url} iconType="transmission" refetchInterval={15000}>
      {(data) => (
        <div className="flex gap-1 px-3 pb-3">
          <Block label="下载中" value={data.downloading ?? 0} />
          <Block label="下载速率" value={formatSpeed(data.dl_speed || 0)} />
          <Block label="做种" value={data.seeding ?? 0} />
          <Block label="上传速率" value={formatSpeed(data.up_speed || 0)} />
        </div>
      )}
    </WidgetFrame>
  )
}

function QBittorrentWidget({ widget }: { widget: Widget }) {
  return (
    <WidgetFrame widget={widget} fallbackDesc={widget.url} iconType="qbittorrent" refetchInterval={15000}>
      {(data) => (
        <div className="flex gap-1 px-3 pb-3">
          <Block label="下载中" value={data.downloading} />
          <Block label="下载速率" value={formatSpeed(data.dl_speed || 0)} />
          <Block label="做种" value={data.seeding} />
          <Block label="上传速率" value={formatSpeed(data.up_speed || 0)} />
        </div>
      )}
    </WidgetFrame>
  )
}

// --- OpenWrt Widget ---
function OpenWrtWidget({ widget }: { widget: Widget }) {
  return (
    <WidgetFrame widget={widget} fallbackDesc="路由器" iconType="openwrt">
      {(data) => (
        <div className="flex gap-1 px-3 pb-3">
          <Block label="CPU" value={data.cpu_load?.toFixed(2)} />
          <Block label="运行天数" value={formatUptimeDays(data.uptime)} />
          <Block label="可用内存" value={formatBytes(data.mem_free)} />
          <Block label="剩余空间" value={formatBytes(data.disk_free)} />
        </div>
      )}
    </WidgetFrame>
  )
}

function formatUptimeDays(seconds: number): string {
  if (!seconds || seconds <= 0) return '-'
  return Math.floor(seconds / 86400) + ' 天'
}

// --- Home Assistant Widget ---
function HomeAssistantWidget({ widget }: { widget: Widget }) {
  return (
    <WidgetFrame widget={widget} fallbackDesc={(d) => d.ha_status || '智能家居'} iconType="homeassistant">
      {(data) => {
        // Build blocks dynamically
        const blocks: { label: string; value: string | number }[] = [
          { label: '在家人数', value: data.people_home ?? 0 },
          { label: '灯', value: data.lights_on ?? 0 },
          { label: '开关', value: data.switches_on ?? 0 },
        ]

        // Add custom entity blocks (up to 3 more to fit in 4-column layout)
        if (data.custom && data.custom.length > 0) {
          for (const ce of data.custom.slice(0, 1)) {
            const val = ce.unit ? `${ce.value} ${ce.unit}` : ce.value
            blocks.push({ label: ce.label, value: val })
          }
        }

        return (
          <>
            <div className="flex gap-1 px-3 pb-3">
              {blocks.map((b, i) => <Block key={i} label={b.label} value={b.value} />)}
            </div>
            {data.custom && data.custom.length > 1 && (
              <div className="px-3 pb-3">
                <div className="text-xs text-white mb-1.5 font-medium">自定义实体</div>
                <div className="flex gap-1">
                  {data.custom.slice(1, 4).map((ce: { entity_id: string; label: string; value: string; unit?: string }, i: number) => (
                    <Block key={i} label={ce.label} value={ce.unit ? `${ce.value} ${ce.unit}` : ce.value} />
                  ))}
                </div>
              </div>
            )}
          </>
        )
      }}
    </WidgetFrame>
  )
}

// --- Shared Components ---
export function WidgetSkeleton({ name }: { name: string }) {
  return (
    <div className="bg-bg-card/70 rounded-xl animate-pulse">
      <div className="flex items-center gap-3 p-4 pb-2">
        <div className="w-8 h-8 rounded bg-bg-hover" />
        <div>
          <div className="h-4 bg-bg-hover rounded w-24 mb-1" />
          <div className="h-3 bg-bg-hover rounded w-32" />
        </div>
      </div>
      <div className="flex gap-1 px-3 pb-3">
        <div className="flex-1 h-12 bg-bg-hover rounded-md" />
        <div className="flex-1 h-12 bg-bg-hover rounded-md" />
        <div className="flex-1 h-12 bg-bg-hover rounded-md" />
        <div className="flex-1 h-12 bg-bg-hover rounded-md" />
      </div>
    </div>
  )
}

export function WidgetError({ name, error }: { name: string; error?: string }) {
  return (
    <div className="bg-bg-card/70 rounded-xl p-4 border border-err/20">
      <div className="flex items-center gap-2">
        <span className="text-lg">⚠️</span>
        <span className="font-semibold text-sm">{name}</span>
        <span className="text-xs text-err/80 ml-auto">API 错误</span>
      </div>
    </div>
  )
}

// --- OpenClash Widget ---
function OpenClashWidget({ widget }: { widget: Widget }) {
  const [switching, setSwitching] = useState(false)
  const [toggling, setToggling] = useState(false)
  const queryClient = useQueryClient()

  const { data, isLoading, error } = useQuery({
    queryKey: ['widget-data', widget.id],
    queryFn: () => widgets.data(widget.id),
    refetchInterval: 10000,
    retry: 1,
  })

  const { data: nodesData } = useQuery({
    queryKey: ['openclash-nodes', widget.id],
    queryFn: () => widgets.openclashNodes(widget.id),
    refetchInterval: 30000,
    retry: 1,
  })

  const { data: statusData } = useQuery({
    queryKey: ['openclash-status', widget.id],
    queryFn: () => widgets.openclashStatus(widget.id),
    refetchInterval: 5000,
    retry: 1,
  })

  const running = statusData?.running ?? true
  const isStopped = statusData?.running === false

  if (isLoading) return <WidgetSkeleton name={widget.name} />
  // Only show error if service is supposed to be running but data fetch failed
  if (error && !isStopped) return <WidgetError name={widget.name} error={(error as Error).message} />

  const trafficUp = formatBytes(data?.traffic_up ?? 0)
  const trafficDown = formatBytes(data?.traffic_down ?? 0)
  const latency = data?.ping_latency != null ? data.ping_latency.toFixed(0) + ' ms' : '—'
  const remaining = data?.remaining_traffic ? data.remaining_traffic.toFixed(0) + ' GB' : '—'
  const node = data?.node || '—'
  const nodes = nodesData?.nodes || []

  const handleSwitch = async (e: React.ChangeEvent<HTMLSelectElement>) => {
    const newNode = e.target.value
    if (!newNode || newNode === node) return
    setSwitching(true)
    try {
      await widgets.openclashSwitch(widget.id, newNode)
      queryClient.invalidateQueries({ queryKey: ['widget-data', widget.id] })
      queryClient.invalidateQueries({ queryKey: ['openclash-nodes', widget.id] })
    } catch (err) {
      console.error('Switch node failed:', err)
    } finally {
      setSwitching(false)
    }
  }

  const handleToggle = async () => {
    const action = running ? 'stop' : 'start'
    setToggling(true)
    try {
      await widgets.openclashControl(widget.id, action)
      // Wait a moment then refresh status and data
      setTimeout(() => {
        queryClient.invalidateQueries({ queryKey: ['openclash-status', widget.id] })
        queryClient.invalidateQueries({ queryKey: ['widget-data', widget.id] })
      }, 2000)
    } catch (err) {
      console.error('Control failed:', err)
    } finally {
      setToggling(false)
    }
  }

  const handleRestart = async () => {
    setToggling(true)
    try {
      await widgets.openclashControl(widget.id, 'restart')
      setTimeout(() => {
        queryClient.invalidateQueries({ queryKey: ['openclash-status', widget.id] })
        queryClient.invalidateQueries({ queryKey: ['widget-data', widget.id] })
      }, 3000)
    } catch (err) {
      console.error('Restart failed:', err)
    } finally {
      setToggling(false)
    }
  }

  return (
    <WidgetCard widget={widget}>
      <div className="flex items-center gap-3 p-4 pb-3">
        <WidgetTitle widget={widget} fallbackDesc="OpenClash 代理"><WidgetIcon type="openclash" /></WidgetTitle>
      </div>
      <div className="px-4 pb-2">
        <div className="text-xs text-white/70 mb-1">当前节点</div>
        {nodes.length > 0 ? (
          <select
            value={node}
            onChange={handleSwitch}
            onClick={(e) => e.stopPropagation()}
            disabled={switching}
            className="w-full bg-white/10 border border-white/20 rounded px-2 py-1 text-sm text-white truncate cursor-pointer hover:bg-white/15 focus:outline-none focus:border-blue-400/50 disabled:opacity-50 disabled:cursor-wait"
          >
            {nodes.map((n: string) => (
              <option key={n} value={n} className="bg-gray-800 text-white">{n}</option>
            ))}
          </select>
        ) : (
          <div className="text-sm font-medium truncate" title={node}>{node}</div>
        )}
      </div>
      <div className="flex gap-1 px-3 py-3 items-stretch">
        {/* Toggle switch */}
        <div className="flex-1 bg-white/5 rounded-lg p-2 flex items-center justify-center gap-2 min-h-[48px]">
          <button
            onClick={handleToggle}
            disabled={toggling}
            className={`relative inline-flex h-6 w-11 items-center rounded-full transition-colors duration-300 focus:outline-none focus:ring-2 focus:ring-blue-400/50 focus:ring-offset-1 focus:ring-offset-transparent ${
              toggling ? 'opacity-50 cursor-wait' : 'cursor-pointer'
            } ${running ? 'bg-emerald-500/80' : 'bg-white/15'}`}
            title={running ? '点击停止 OpenClash' : '点击启动 OpenClash'}
          >
            <span
              className={`inline-block h-4 w-4 transform rounded-full bg-white shadow-sm transition-transform duration-300 ${
                running ? 'translate-x-[22px]' : 'translate-x-[4px]'
              }`}
            />
          </button>
          <span className={`text-xs font-medium min-w-[40px] text-center ${running ? 'text-emerald-400' : 'text-white/50'}`}>
            {toggling ? '…' : running ? '运行中' : '已停止'}
          </span>
        </div>
        {/* Restart button */}
        <div className="flex-1 bg-white/5 rounded-lg p-2 flex items-center justify-center min-h-[48px]">
          <button
            onClick={handleRestart}
            disabled={toggling || !running}
            className="text-xs text-white/70 hover:text-white disabled:opacity-40 disabled:cursor-not-allowed w-full h-full rounded transition-colors hover:bg-white/10"
            title="重启 OpenClash"
          >
            <svg xmlns="http://www.w3.org/2000/svg" className={`h-6 w-6 inline mr-1 ${running ? 'text-emerald-400' : ''}`} fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2}>
              <path strokeLinecap="round" strokeLinejoin="round" d="M4 4v5h.582m15.356 2A8.001 8.001 0 004.582 9m0 0H9m11 11v-5h-.581m0 0a8.003 8.003 0 01-15.357-2m15.357 2H15" />
            </svg>
            重启
          </button>
        </div>
      </div>
      <div className="flex gap-1 px-3 pb-7">
        <Block label="上传总量" value={trafficUp} />
        <Block label="下载总量" value={trafficDown} />
        <Block label="延迟" value={latency} />
        <Block label="剩余流量" value={remaining} />
      </div>
    </WidgetCard>
  )
}

// --- iKuai Widget ---
function IkuaiWidget({ widget }: { widget: Widget }) {
  const { data, isLoading, error } = useQuery({
    queryKey: ['widget-data', widget.id],
    queryFn: () => widgets.data(widget.id),
    refetchInterval: 15000,
    retry: 1,
  })

  if (isLoading) return <WidgetSkeleton name={widget.name} />
  if (error) return <WidgetError name={widget.name} error={(error as Error).message} />

  const download = data.download >= 1024
    ? (data.download / 1024).toFixed(1) + ' MB/s'
    : (data.download || 0).toFixed(1) + ' KB/s'
  const upload = data.upload >= 1024
    ? (data.upload / 1024).toFixed(1) + ' MB/s'
    : (data.upload || 0).toFixed(1) + ' KB/s'

  return (
    <WidgetCard widget={widget}>
      <div className="flex items-center gap-3 p-4 pb-2">
        <WidgetTitle widget={widget} fallbackDesc="爱快路由器"><WidgetIcon type="ikuai" /></WidgetTitle>
      </div>
      <div className="flex gap-1 px-3 pb-3">
        <Block label="CPU" value={(data.cpu || 0).toFixed(1) + ' %'} />
        <Block label="客户端" value={data.clients ?? 0} />
        <Block label="下行速率" value={download} />
        <Block label="上行速率" value={upload} />
      </div>
    </WidgetCard>
  )
}

// --- Widget Renderer ---
export function renderWidget(widget: Widget) {
  switch (widget.type) {
    case 'proxmox': return <ProxmoxWidget key={widget.id} widget={widget} />
    case 'pbs': return <PBSWidget key={widget.id} widget={widget} />
    case 'unraid': return <UnraidWidget key={widget.id} widget={widget} />
    case 'portainer': return <PortainerWidget key={widget.id} widget={widget} />
    case 'adguard': return <AdGuardWidget key={widget.id} widget={widget} />
    case 'jellyfin': return <JellyfinWidget key={widget.id} widget={widget} />
    case 'moviepilot': return <MoviePilotWidget key={widget.id} widget={widget} />
    case 'qbittorrent': return <QBittorrentWidget key={widget.id} widget={widget} />
    case 'hawkeye': return <HawkeyeWidget key={widget.id} widget={widget} />
    case 'lucky': return <LuckyWidget key={widget.id} widget={widget} />
    case 'transmission': return <TransmissionWidget key={widget.id} widget={widget} />
    case 'homeassistant': return <HomeAssistantWidget key={widget.id} widget={widget} />
    case 'openwrt': return <OpenWrtWidget key={widget.id} widget={widget} />
    case 'ikuai': return <IkuaiWidget key={widget.id} widget={widget} />
    case 'openclash': return <OpenClashWidget key={widget.id} widget={widget} />
    default: return <WidgetError key={widget.id} name={widget.name} error={`未知类型: ${widget.type}`} />
  }
}
