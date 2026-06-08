import { useState, useEffect, useMemo } from 'react'
import { Routes, Route, NavLink, Outlet, useNavigate } from 'react-router-dom'
import { versionInfo, auth } from '../api/client'
import { toCSTFull } from '../utils'
import BgSettingsPanel from './BgSettingsPanel'

const navItems = [
  { to: '/', label: '仪表盘', icon: '📊' },
  { to: '/hosts', label: '主机状态', icon: '🖥️' },
  { to: '/services', label: '网站状态', icon: '🌐' },
  { to: '/widgets', label: '服务组件', icon: '🧩' },
  { to: '/monitors', label: '监控管理', icon: '📡' },
  { to: '/alerts', label: '警告通知', icon: '🔔' },
]

interface SessionInfo {
  id: number
  user_agent: string
  ip_address: string
  created_at: string
  is_current: boolean
}

function SessionsDialog({ onClose }: { onClose: () => void }) {
  const [sessions, setSessions] = useState<SessionInfo[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState('')

  useEffect(() => {
    auth.sessions()
      .then(r => setSessions(r.sessions))
      .catch(e => setError(e.message))
      .finally(() => setLoading(false))
  }, [])

  const handleKick = async (id: number) => {
    try {
      await auth.deleteSession(id)
      setSessions(prev => prev.filter(s => s.id !== id))
    } catch (e: any) {
      setError(e.message)
    }
  }

  const parseUA = (ua: string) => {
    if (!ua) return ''
    if (ua.includes('Windows')) return 'Windows'
    if (ua.includes('Mac OS')) return 'macOS'
    if (ua.includes('Linux')) return 'Linux'
    if (ua.includes('Android')) return 'Android'
    if (ua.includes('iPhone') || ua.includes('iPad')) return 'iOS'
    return ''
  }

  const formatTime = (ts: string) => {
    if (!ts) return ''
    try {
      return toCSTFull(ts)
    } catch { return ts }
  }

  return (
    <div style={{
      position: 'fixed', inset: 0, zIndex: 100,
      display: 'flex', alignItems: 'center', justifyContent: 'center',
    }}>
      <div onClick={onClose} style={{ position: 'absolute', inset: 0, background: 'rgba(0,0,0,0.6)' }} />
      <div style={{
        position: 'relative', width: 360, maxWidth: '90vw', maxHeight: '80vh', overflow: 'auto',
        background: '#0f0f23', border: '1px solid #7c3aed33', borderRadius: 12, padding: 24,
      }}>
        <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: 16 }}>
          <h3 style={{ color: '#e2e8f0', fontSize: 16, margin: 0 }}>已登录设备</h3>
          <button onClick={onClose} style={{ background: 'none', border: 'none', color: '#666', cursor: 'pointer', fontSize: 18 }}>
            ✕
          </button>
        </div>

        {loading && <div style={{ color: '#666', fontSize: 13, textAlign: 'center', padding: 20 }}>加载中...</div>}

        {error && (
          <div style={{ color: '#ef4444', fontSize: 12, padding: '8px 12px', background: '#1a0a0a', borderRadius: 6, marginBottom: 12 }}>
            {error}
          </div>
        )}

        {!loading && sessions.length === 0 && (
          <div style={{ color: '#666', fontSize: 13, textAlign: 'center', padding: 20 }}>暂无活跃会话</div>
        )}

        {!loading && sessions.map((s, i) => (
          <div key={s.id} style={{
            display: 'flex', alignItems: 'center', justifyContent: 'space-between',
            padding: '10px 0', borderBottom: i < sessions.length - 1 ? '1px solid #7c3aed11' : 'none',
          }}>
            <div>
              <div style={{ color: '#c4b5fd', fontSize: 13, display: 'flex', alignItems: 'center', gap: 6 }}>
                {s.ip_address || '未知IP'}
                {s.is_current && (
                  <span style={{ fontSize: 10, color: '#7c3aed', background: '#7c3aed15', padding: '1px 6px', borderRadius: 4 }}>
                    当前
                  </span>
                )}
              </div>
              <div style={{ color: '#666', fontSize: 11, marginTop: 2 }}>
                {[parseUA(s.user_agent), formatTime(s.created_at)].filter(Boolean).join(' · ')}
              </div>
            </div>
            {!s.is_current && (
              <button
                onClick={() => handleKick(s.id)}
                style={{
                  background: 'transparent', border: '1px solid #ef444433', borderRadius: 6,
                  color: '#ef4444', fontSize: 11, cursor: 'pointer', padding: '3px 10px',
                }}
              >
                退出
              </button>
            )}
          </div>
        ))}
      </div>
    </div>
  )
}

export default function Layout() {
  const navigate = useNavigate()
  const [menuOpen, setMenuOpen] = useState(false)
  const [serverVer, setServerVer] = useState('')
  const [showSessions, setShowSessions] = useState(false)

  // 背景图片
  const bgPresets = useMemo(() => [
    '/preset_bg/bg1.jpg',
    '/preset_bg/bg2.jpg',
    '/preset_bg/bg3.jpg',
    '/preset_bg/bg4.jpg',
    '/preset_bg/bg5.jpg',
  ], [])
  const [bgIndex, setBgIndex] = useState<number>(() => {
    const stored = localStorage.getItem('hawkeye-bg-index')
    return stored != null ? parseInt(stored, 10) : -1
  })
  const [bgCustomUrl, setBgCustomUrl] = useState<string>(() => {
    return localStorage.getItem('hawkeye-bg-custom-url') || ''
  })
  const [bgPanelOpen, setBgPanelOpen] = useState(false)

  const handleSelectBg = (idx: number) => {
    setBgIndex(idx)
    localStorage.setItem('hawkeye-bg-index', String(idx))
    if (idx !== -2) {
      setBgCustomUrl('')
      localStorage.removeItem('hawkeye-bg-custom-url')
    }
  }

  const handleSelectCustom = (url: string) => {
    setBgCustomUrl(url)
    if (url) {
      localStorage.setItem('hawkeye-bg-custom-url', url)
      setBgIndex(-2)
      localStorage.setItem('hawkeye-bg-index', '-2')
    }
  }

  useEffect(() => {
    versionInfo.get().then(r => setServerVer(r.version)).catch(() => {})
  }, [])

  const handleLogout = () => {
    localStorage.removeItem('auth_token')
    localStorage.removeItem('username')
    navigate('/')
    window.location.reload()
  }

  const handleNav = () => {
    setMenuOpen(false)
  }

  const hasBgImage = (bgIndex >= 0 && bgIndex < bgPresets.length) || (bgIndex === -2 && bgCustomUrl !== '')
  const bgImageUrl = bgIndex === -2 ? bgCustomUrl : (bgIndex >= 0 && bgIndex < bgPresets.length ? bgPresets[bgIndex] : '')

  // 主内容区背景：有背景图时更透明，让图片透出来
  const mainBgStyle = hasBgImage
    ? {
        backgroundImage: `
          linear-gradient(135deg,
            rgba(var(--main-r), var(--main-g), var(--main-b), 0.10) 0%,
            rgba(var(--bg-r), var(--bg-g), var(--bg-b), 0.25) 50%,
            rgba(var(--end-r), var(--end-g), var(--end-b), 0.10) 100%)
        `,
        backgroundColor: 'rgba(var(--bg-body), 0.3)',
      }
    : {
        backgroundImage: `
          linear-gradient(135deg,
            rgba(var(--main-r), var(--main-g), var(--main-b), 0.3) 0%,
            rgba(var(--main-r), var(--main-g), var(--main-b), 0.2) 12%,
            rgba(var(--bg-r), var(--bg-g), var(--bg-b), 0.85) 25%,
            rgba(var(--bg-r), var(--bg-g), var(--bg-b), 0.9) 50%,
            rgba(var(--end-r), var(--end-g), var(--end-b), 0.2) 80%,
            rgba(var(--end-r), var(--end-g), var(--end-b), 0.3) 100%)
        `,
        backgroundColor: 'rgb(var(--bg-body))',
      }

  return (
    <>
      {/* 主布局 — 背景图直接挂在这里，overlay 作为第一个子元素 */}
      <div
        className="layout-outer flex h-screen overflow-hidden"
        style={hasBgImage ? {
          backgroundImage: `url('${bgImageUrl}')`,
          backgroundSize: 'cover',
          backgroundPosition: 'center',
          backgroundRepeat: 'no-repeat',
        } : undefined}
      >
      {/* 背景暗色覆盖层 */}
      {hasBgImage && <div className="bg-image-overlay-inner" />}

      {/* Sessions dialog */}
      {showSessions && <SessionsDialog onClose={() => setShowSessions(false)} />}

      {/* Mobile overlay */}
      {menuOpen && (
        <div className="fixed inset-0 bg-black/50 z-40 md:hidden" onClick={() => setMenuOpen(false)} />
      )}

      {/* Sidebar — 与主内容区统一背景，右侧竖线分隔 */}
      <aside
        className={`
          fixed md:static inset-y-0 left-0 z-50
          w-56 flex flex-col shrink-0
          transform transition-transform duration-200 ease-in-out
          border-r border-white/[0.06]
          ${menuOpen ? 'translate-x-0' : '-translate-x-full'} md:translate-x-0
        `}
        style={mainBgStyle}
      >
        <div className="flex items-center justify-between px-5 py-4">
          <div>
            <h1 className="text-2xl font-extrabold gradient-text">Hawkeye</h1>
            <p className="text-xs text-gray-500 mt-0.5">系统监控</p>
          </div>
          <button onClick={() => setMenuOpen(false)} className="md:hidden text-gray-400 hover:text-gray-200 p-1">
            <svg className="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
              <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M6 18L18 6M6 6l12 12" />
            </svg>
          </button>
        </div>
        <nav className="flex-1 py-3 px-2 space-y-1">
          {navItems.map((item) => (
            <NavLink
              key={item.to}
              to={item.to}
              end={item.to === '/'}
              onClick={handleNav}
              className={({ isActive }) =>
                `flex items-center gap-3 px-3 py-2.5 rounded-lg text-base transition-colors ${
                  isActive ? 'text-white bg-white/10 font-semibold' : 'text-gray-400 hover:text-gray-200 hover:bg-bg-hover'
                }`
              }
            >
              <span className="text-base">{item.icon}</span>
              {item.label}
            </NavLink>
          ))}
          {/* 背景设置入口 */}
          <button
            onClick={() => setBgPanelOpen(true)}
            className={`flex items-center gap-3 px-3 py-2.5 rounded-lg text-base transition-colors w-full text-left mt-2 border-t border-white/5 pt-3 ${
              bgPanelOpen ? 'text-white bg-white/10 font-semibold' : 'text-gray-400 hover:text-gray-200 hover:bg-bg-hover'
            }`}
          >
            <span className="text-base">🎨</span>
            背景设置
          </button>
        </nav>
        <div className="px-5 py-3 space-y-2">
          <button
            onClick={() => setShowSessions(true)}
            className="w-full text-left text-sm text-gray-500 hover:text-accent transition-colors"
          >
            ⬡ 已登录设备
          </button>
          <button
            onClick={handleLogout}
            className="w-full text-left text-sm text-gray-500 hover:text-red-400 transition-colors"
          >
            &larr; 退出
          </button>
          <span className="text-sm text-gray-500 block">ver.{serverVer || '...'}</span>
        </div>
      </aside>

      {/* Main Content */}
      <div className="flex-1 flex flex-col overflow-hidden">
        {/* Mobile header bar */}
        <header className="md:hidden flex items-center gap-3 px-4 py-3 bg-bg-card/70 shrink-0">
          <button onClick={() => setMenuOpen(true)} className="text-gray-400 hover:text-gray-200 p-1">
            <svg className="w-6 h-6" fill="none" stroke="currentColor" viewBox="0 0 24 24">
              <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M4 6h16M4 12h16M4 18h16" />
            </svg>
          </button>
          <span className="text-sm font-semibold text-accent">Hawkeye</span>
        </header>
        <main className="flex-1 overflow-auto" style={mainBgStyle}>
          <Outlet />
        </main>
      </div>
    </div>

    {/* 背景设置面板 */}
    <BgSettingsPanel
      open={bgPanelOpen}
      onClose={() => setBgPanelOpen(false)}
      selectedBg={bgIndex}
      onSelectBg={handleSelectBg}
      selectedCustomUrl={bgCustomUrl}
      onSelectCustom={handleSelectCustom}
    />
    </>
  )
}
