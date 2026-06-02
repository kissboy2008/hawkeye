import { useQuery } from '@tanstack/react-query'
import { widgets, type Widget } from '../api/client'

// --- Widget Icons ---
const WIDGET_ICONS: Record<string, string> = {
  proxmox: '/icons/proxmox.png',
  pbs: '/icons/pbs.png',
  unraid: '/icons/unraid.png',
  portainer: '/icons/portainer.png',
  adguard: '/icons/adguard.png',
  jellyfin: '/icons/jellyfin.png',
  moviepilot: '/icons/moviepilot.png',
  qbittorrent: '/icons/qbittorrent.png',
  hawkeye: '/icons/hawkeye.svg',
  lucky: '/icons/lucky.svg',
  transmission: '/icons/transmission.png',
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
      <div className="text-sm text-gray-100 font-light tabular-nums truncate w-full">
        {value === undefined || value === null ? '-' : value}
      </div>
      <div className="text-xs font-bold uppercase text-gray-400 truncate w-full">{label}</div>
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
        <div className="text-xs text-gray-400 truncate">{widget.description || fallbackDesc || ''}</div>
      </div>
    </div>
  )
}

function WidgetCard({ widget, children }: { widget: Widget; children: React.ReactNode }) {
  const linkUrl = (() => { try { return JSON.parse(widget.config || '{}').link_url } catch { return '' } })()
  return (
    <div
      className={`bg-bg-card rounded-xl border border-purple-500/10 shadow-soft hover:shadow-glow transition-shadow ${linkUrl ? 'cursor-pointer' : ''}`}
      onClick={() => { if (linkUrl) window.open(linkUrl, '_blank', 'noopener,noreferrer') }}
    >
      {children}
    </div>
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
          <div className="text-xs text-gray-500 mb-1.5 font-medium">实时会话</div>
          <div className="space-y-1">
            {data.sessions.filter((s: { now_playing?: string }) => s.now_playing).map((s: { user_name: string; now_playing?: string; progress_ticks?: number; runtime_ticks?: number; is_paused?: boolean }, i: number) => (
              <div key={i} className="bg-white/5 rounded-md px-2.5 py-2 text-xs">
                <div className="flex items-center justify-between mb-1.5">
                  <span className="text-gray-200 font-medium truncate">{s.now_playing} ({s.user_name})</span>
                  {s.runtime_ticks && s.runtime_ticks > 0 && (
                    <span className="text-gray-400 flex-shrink-0 ml-2 tabular-nums">
                      {formatTicks(s.progress_ticks || 0)} / {formatTicks(s.runtime_ticks)}
                    </span>
                  )}
                </div>
                <div className="flex items-center gap-2">
                  <span className="text-gray-400">{s.is_paused ? '⏸' : '▶'}</span>
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

// --- Shared Components ---
export function WidgetSkeleton({ name }: { name: string }) {
  return (
    <div className="bg-bg-card rounded-xl border border-purple-500/10 animate-pulse">
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
    <div className="bg-bg-card rounded-xl p-4 border border-err/20">
      <div className="flex items-center gap-2">
        <span className="text-lg">⚠️</span>
        <span className="font-semibold text-sm">{name}</span>
        <span className="text-xs text-err/80 ml-auto">API 错误</span>
      </div>
    </div>
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
    default: return <WidgetError key={widget.id} name={widget.name} error={`未知类型: ${widget.type}`} />
  }
}
