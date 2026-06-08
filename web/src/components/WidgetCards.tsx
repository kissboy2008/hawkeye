import { useState } from 'react'
import { useQuery } from '@tanstack/react-query'
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

function WidgetCard({ widget, children }: { widget: Widget; children: React.ReactNode }) {
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
        className={`bg-bg-card/70 rounded-xl shadow-soft hover:shadow-glow transition-shadow ${linkUrl ? 'cursor-pointer' : ''}`}
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

// --- Proxmox VE Widget ---
function ProxmoxWidget({ widget }: { widget: Widget }) {
  const { data, isLoading, error } = useQuery({
    queryKey: ['widget-data', widget.id],
    queryFn: () => widgets.data(widget.id),
    refetchInterval: 30000,
    retry: 1,
  })

  if (isLoading) return <WidgetSkeleton name={widget.name} />
  if (error) return <WidgetError name={widget.name} error={(error as Error).message} />

  const cpuPct = ((data.cpu || 0) * 100).toFixed(0) + '%'
  const memPct = data.maxmem > 0 ? ((data.mem / data.maxmem) * 100).toFixed(0) + '%' : '0%'
  const runningVMs = data.vms?.filter((v: any) => v.status === 'running').length || 0
  const totalVMs = data.vms?.length || 0
  const runningCTs = data.cts?.filter((c: any) => c.status === 'running').length || 0
  const totalCTs = data.cts?.length || 0

  return (
    <WidgetCard widget={widget}>
      <div className="flex items-center gap-3 p-4 pb-2">
        <WidgetTitle widget={widget} fallbackDesc="Simplify your Data Center"><WidgetIcon type="proxmox" /></WidgetTitle>
        <div className="ml-auto"><PingBadge pingMs={data.ping_ms} /></div>
      </div>
      <div className="flex gap-1 px-3 pb-3">
        <Block label="CPU" value={cpuPct} />
        <Block label="内存" value={memPct} />
        <Block label="虚拟机" value={`${runningVMs} / ${totalVMs}`} />
        <Block label="容器" value={`${runningCTs} / ${totalCTs}`} />
      </div>
    </WidgetCard>
  )
}

// --- PBS Widget ---
function PBSWidget({ widget }: { widget: Widget }) {
  const { data, isLoading, error } = useQuery({
    queryKey: ['widget-data', widget.id],
    queryFn: () => widgets.data(widget.id),
    refetchInterval: 60000,
  })

  if (isLoading) return <WidgetSkeleton name={widget.name} />
  if (error) return <WidgetError name={widget.name} error={(error as Error).message} />

  const usedPct = (data.used_percent || 0).toFixed(0) + '%'
  const cpuPct = ((data.cpu || 0) * 100).toFixed(0) + '%'
  const memPct = data.maxmem > 0 ? ((data.mem / data.maxmem) * 100).toFixed(0) + '%' : '0%'

  return (
    <WidgetCard widget={widget}>
      <div className="flex items-center gap-3 p-4 pb-2">
        <WidgetTitle widget={widget} fallbackDesc="Enterprise Backup Solution"><WidgetIcon type="pbs" /></WidgetTitle>
        <div className="ml-auto"><PingBadge pingMs={data.ping_ms} /></div>
      </div>
      <div className="flex gap-1 px-3 pb-3">
        <Block label="CPU" value={cpuPct} />
        <Block label="内存" value={memPct} />
        <Block label="数据存储" value={usedPct} />
        <Block label="快照数量" value={data.snapshots || 0} />
      </div>
    </WidgetCard>
  )
}

// --- Unraid Widget ---
function UnraidWidget({ widget }: { widget: Widget }) {
  const { data, isLoading, error } = useQuery({
    queryKey: ['widget-data', widget.id],
    queryFn: () => widgets.data(widget.id),
    refetchInterval: 30000,
    retry: 1,
  })

  if (isLoading) return <WidgetSkeleton name={widget.name} />
  if (error) return <WidgetError name={widget.name} error={(error as Error).message} />

  const cpuPct = (data.cpu || 0).toFixed(0) + '%'
  const memPct = (data.mem_percent || 0).toFixed(0) + '%'
  const status = data.array_state === 'STARTED' ? 'Started' : data.array_state

  return (
    <WidgetCard widget={widget}>
      <div className="flex items-center gap-3 p-4 pb-2">
        <WidgetTitle widget={widget} fallbackDesc="网络附加存储"><WidgetIcon type="unraid" /></WidgetTitle>
        <div className="ml-auto"><PingBadge pingMs={data.ping_ms} /></div>
      </div>
      <div className="flex gap-1 px-3 pb-3">
        <Block label="CPU" value={cpuPct} />
        <Block label="内存" value={memPct} />
        <Block label="状态" value={status} />
        <Block label="通知" value={0} />
      </div>
    </WidgetCard>
  )
}

// --- Portainer Widget ---
function PortainerWidget({ widget }: { widget: Widget }) {
  const { data, isLoading, error } = useQuery({
    queryKey: ['widget-data', widget.id],
    queryFn: () => widgets.data(widget.id),
    refetchInterval: 30000,
    retry: 1,
  })

  if (isLoading) return <WidgetSkeleton name={widget.name} />
  if (error) return <WidgetError name={widget.name} error={(error as Error).message} />

  return (
    <WidgetCard widget={widget}>
      <div className="flex items-center gap-3 p-4 pb-2">
        <WidgetTitle widget={widget} fallbackDesc="容器管理"><WidgetIcon type="portainer" /></WidgetTitle>
        <div className="ml-auto"><PingBadge pingMs={data.ping_ms} /></div>
      </div>
      <div className="flex gap-1 px-3 pb-3">
        <Block label="运行中" value={data.running} />
        <Block label="已停止" value={data.stopped} />
        <Block label="总计" value={data.total} />
      </div>
    </WidgetCard>
  )
}

// --- AdGuard Home Widget ---
function AdGuardWidget({ widget }: { widget: Widget }) {
  const { data, isLoading, error } = useQuery({
    queryKey: ['widget-data', widget.id],
    queryFn: () => widgets.data(widget.id),
    refetchInterval: 60000,
  })

  if (isLoading) return <WidgetSkeleton name={widget.name} />
  if (error) return <WidgetError name={widget.name} error={(error as Error).message} />

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
    <WidgetCard widget={widget}>
      <div className="flex items-center gap-3 p-4 pb-2">
        <WidgetTitle widget={widget} fallbackDesc="DNS 广告过滤"><WidgetIcon type="adguard" /></WidgetTitle>
        <div className="ml-auto"><PingBadge pingMs={data.ping_ms} /></div>
      </div>
      <div className="flex gap-1 px-3 pb-3">
        <Block label="查询数" value={queries} />
        <Block label="已拦截" value={blocked} />
        <Block label="过滤" value={data.filtered} />
        <Block label="延迟" value={avgMs} />
      </div>
    </WidgetCard>
  )
}

// --- Jellyfin Widget ---
function JellyfinWidget({ widget }: { widget: Widget }) {
  const { data, isLoading, error } = useQuery({
    queryKey: ['widget-data', widget.id],
    queryFn: () => widgets.data(widget.id),
    refetchInterval: 30000,
    retry: 1,
  })

  if (isLoading) return <WidgetSkeleton name={widget.name} />
  if (error) return <WidgetError name={widget.name} error={(error as Error).message} />

  return (
    <WidgetCard widget={widget}>
      <div className="flex items-center gap-3 p-4 pb-2">
        <WidgetTitle widget={widget} fallbackDesc={data.status || '媒体服务器'}><WidgetIcon type="jellyfin" /></WidgetTitle>
        <div className="ml-auto"><PingBadge pingMs={data.ping_ms} /></div>
      </div>
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
    </WidgetCard>
  )
}

// --- MoviePilot Widget ---
function MoviePilotWidget({ widget }: { widget: Widget }) {
  const { data, isLoading, error } = useQuery({
    queryKey: ['widget-data', widget.id],
    queryFn: () => widgets.data(widget.id),
    refetchInterval: 60000,
  })

  if (isLoading) return <WidgetSkeleton name={widget.name} />
  if (error) return <WidgetError name={widget.name} error={(error as Error).message} />

  return (
    <WidgetCard widget={widget}>
      <div className="flex items-center gap-3 p-4 pb-2">
        <WidgetTitle widget={widget} fallbackDesc="自动化媒体管理"><WidgetIcon type="moviepilot" /></WidgetTitle>
        <div className="ml-auto"><PingBadge pingMs={data.ping_ms} /></div>
      </div>
      <div className="flex gap-1 px-3 pb-3">
        <Block label="电影订阅" value={data.movie_subscribes} />
        <Block label="电视剧订阅" value={data.tv_subscribes} />
        <Block label="总空间" value={data.total_storage} />
        <Block label="剩余空间" value={data.free_storage} />
      </div>
    </WidgetCard>
  )
}

// --- Hawkeye Self Status Widget ---
function HawkeyeWidget({ widget }: { widget: Widget }) {
  const { data, isLoading, error } = useQuery({
    queryKey: ['widget-data', widget.id],
    queryFn: () => widgets.data(widget.id),
    refetchInterval: 10000,
  })

  if (isLoading) return <WidgetSkeleton name={widget.name} />
  if (error) return <WidgetError name={widget.name} error={(error as Error).message} />

  return (
    <WidgetCard widget={widget}>
      <div className="flex items-center gap-3 p-4 pb-2">
        <WidgetTitle widget={widget} fallbackDesc="系统监控中心"><WidgetIcon type="hawkeye" /></WidgetTitle>
        <div className="ml-auto"><PingBadge pingMs={data.ping_ms} /></div>
      </div>
      <div className="flex gap-1 px-3 pb-3">
        <Block label="在线机器" value={`${data.online_agents}/${data.total_agents}`} />
        <Block label="在线网站" value={`${data.online_probes}/${data.total_probes}`} />
        <Block label="警告" value={data.alerts} />
      </div>
    </WidgetCard>
  )
}

// --- Lucky Widget ---
function LuckyWidget({ widget }: { widget: Widget }) {
  const { data, isLoading, error } = useQuery({
    queryKey: ['widget-data', widget.id],
    queryFn: () => widgets.data(widget.id),
    refetchInterval: 15000,
  })

  if (isLoading) return <WidgetSkeleton name={widget.name} />
  if (error) return <WidgetError name={widget.name} error={(error as Error).message} />

  return (
    <WidgetCard widget={widget}>
      <div className="flex items-center gap-3 p-4 pb-2">
        <WidgetTitle widget={widget} fallbackDesc={widget.url}><WidgetIcon type="lucky" /></WidgetTitle>
        <div className="ml-auto flex items-center gap-2">
          <PingBadge pingMs={data.ping_ms} />
        </div>
      </div>
      <div className="flex gap-1 px-3 pb-3">
        <Block label="CPU" value={data.cpu || '--'} />
        <Block label="反代规则" value={data.rules_count != null ? `${data.enabled_count}/${data.sub_rules_count}` : '--'} />
        <Block label="上传" value={data.net_out_speed || '--'} />
        <Block label="下载" value={data.net_in_speed || '--'} />
      </div>
    </WidgetCard>
  )
}

function TransmissionWidget({ widget }: { widget: Widget }) {
  const { data, isLoading, error } = useQuery({
    queryKey: ['widget-data', widget.id],
    queryFn: () => widgets.data(widget.id),
    refetchInterval: 15000,
  })

  if (isLoading) return <WidgetSkeleton name={widget.name} />
  if (error) return <WidgetError name={widget.name} error={(error as Error).message} />

  const dlSpeed = formatSpeed(data.dl_speed || 0)
  const upSpeed = formatSpeed(data.up_speed || 0)

  return (
    <WidgetCard widget={widget}>
      <div className="flex items-center gap-3 p-4 pb-2">
        <WidgetTitle widget={widget} fallbackDesc={widget.url}><WidgetIcon type="transmission" /></WidgetTitle>
        <div className="ml-auto flex items-center gap-2">
          <PingBadge pingMs={data.ping_ms} />
        </div>
      </div>
      <div className="flex gap-1 px-3 pb-3">
        <Block label="下载中" value={data.downloading ?? 0} />
        <Block label="下载速率" value={dlSpeed} />
        <Block label="做种" value={data.seeding ?? 0} />
        <Block label="上传速率" value={upSpeed} />
      </div>
    </WidgetCard>
  )
}

function QBittorrentWidget({ widget }: { widget: Widget }) {
  const { data, isLoading, error } = useQuery({
    queryKey: ['widget-data', widget.id],
    queryFn: () => widgets.data(widget.id),
    refetchInterval: 15000,
  })

  if (isLoading) return <WidgetSkeleton name={widget.name} />
  if (error) return <WidgetError name={widget.name} error={(error as Error).message} />

  const dlSpeed = formatSpeed(data.dl_speed || 0)
  const upSpeed = formatSpeed(data.up_speed || 0)

  return (
    <WidgetCard widget={widget}>
      <div className="flex items-center gap-3 p-4 pb-2">
        <WidgetTitle widget={widget} fallbackDesc={widget.url}><WidgetIcon type="qbittorrent" /></WidgetTitle>
        <div className="ml-auto flex items-center gap-2">
          <PingBadge pingMs={data.ping_ms} />
        </div>
      </div>
      <div className="flex gap-1 px-3 pb-3">
        <Block label="下载中" value={data.downloading} />
        <Block label="下载速率" value={dlSpeed} />
        <Block label="做种" value={data.seeding} />
        <Block label="上传速率" value={upSpeed} />
      </div>
    </WidgetCard>
  )
}

// --- OpenWrt Widget ---
function OpenWrtWidget({ widget }: { widget: Widget }) {
  const { data, isLoading, error } = useQuery({
    queryKey: ['widget-data', widget.id],
    queryFn: () => widgets.data(widget.id),
    refetchInterval: 30000,
    retry: 1,
  })

  if (isLoading) return <WidgetSkeleton name={widget.name} />
  if (error) return <WidgetError name={widget.name} error={(error as Error).message} />

  return (
    <WidgetCard widget={widget}>
      <div className="flex items-center gap-3 p-4 pb-2">
        <WidgetTitle widget={widget} fallbackDesc="路由器">
          <WidgetIcon type="openwrt" />
        </WidgetTitle>
        <div className="ml-auto"><PingBadge pingMs={data.ping_ms} /></div>
      </div>
      <div className="flex gap-1 px-3 pb-3">
        <Block label="CPU" value={data.cpu_load?.toFixed(2)} />
        <Block label="运行天数" value={formatUptimeDays(data.uptime)} />
        <Block label="可用内存" value={formatBytes(data.mem_free)} />
        <Block label="剩余空间" value={formatBytes(data.disk_free)} />
      </div>
    </WidgetCard>
  )
}

function formatUptimeDays(seconds: number): string {
  if (!seconds || seconds <= 0) return '-'
  return Math.floor(seconds / 86400) + ' 天'
}

// --- Home Assistant Widget ---
function HomeAssistantWidget({ widget }: { widget: Widget }) {
  const { data, isLoading, error } = useQuery({
    queryKey: ['widget-data', widget.id],
    queryFn: () => widgets.data(widget.id),
    refetchInterval: 30000,
    retry: 1,
  })

  if (isLoading) return <WidgetSkeleton name={widget.name} />
  if (error) return <WidgetError name={widget.name} error={(error as Error).message} />

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
    <WidgetCard widget={widget}>
      <div className="flex items-center gap-3 p-4 pb-2">
        <WidgetTitle widget={widget} fallbackDesc={data.ha_status || '智能家居'}><WidgetIcon type="homeassistant" /></WidgetTitle>
        <div className="ml-auto"><PingBadge pingMs={data.ping_ms} /></div>
      </div>
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
    </WidgetCard>
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
  const { data, isLoading, error } = useQuery({
    queryKey: ['widget-data', widget.id],
    queryFn: () => widgets.data(widget.id),
    refetchInterval: 10000,
    retry: 1,
  })

  if (isLoading) return <WidgetSkeleton name={widget.name} />
  if (error) return <WidgetError name={widget.name} error={(error as Error).message} />

  const downTotal = formatBytes(data.traffic_down_total || 0)
  const upTotal = formatBytes(data.traffic_up_total || 0)
  const remaining = data.remaining_traffic ? data.remaining_traffic.toFixed(0) + ' GB' : '—'
  const expire = data.expire_date || '—'
  const node = data.node || '—'
  const nodeCount = data.all_nodes_count || 0

  return (
    <WidgetCard widget={widget}>
      <div className="flex items-center gap-3 p-4 pb-2">
        <WidgetTitle widget={widget} fallbackDesc="OpenClash 代理"><WidgetIcon type="openclash" /></WidgetTitle>
      </div>
      <div className="px-4 pb-1">
        <div className="text-xs text-white/70 mb-1">当前节点</div>
        <div className="text-sm font-medium truncate" title={node}>{node}</div>
      </div>
      <div className="flex gap-1 px-3 py-2">
        <Block label="下载总量" value={downTotal} />
        <Block label="上传总量" value={upTotal} />
      </div>
      <div className="flex gap-1 px-3 pb-3">
        <Block label="剩余流量" value={remaining} />
        <Block label="到期时间" value={expire} />
        <Block label="节点数" value={nodeCount} />
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
