import { useState, useEffect } from 'react'
import { Routes, Route, NavLink, Outlet, useNavigate } from 'react-router-dom'
import { versionInfo } from '../api/client'

type Theme = 'cosmic' | 'sunset' | 'ocean' | 'neon'

const themes: { key: Theme; label: string; from: string; to: string }[] = [
  { key: 'cosmic',  label: '星际紫',  from: '#8b5cf6', to: '#f59e0b' },
  { key: 'sunset',  label: '日落橙',  from: '#f97316', to: '#ec4899' },
  { key: 'ocean',   label: '深海蓝',  from: '#0ea5e9', to: '#8b5cf6' },
  { key: 'neon',    label: '霓虹绿',  from: '#10b981', to: '#06b6d4' },
]

const navItems = [
  { to: '/', label: '仪表盘', icon: '◉' },
  { to: '/hosts', label: '主机状态', icon: '▣' },
  { to: '/services', label: '网站状态', icon: '◈' },
  { to: '/widgets', label: '服务组件', icon: '⬡' },
  { to: '/monitors', label: '监控管理', icon: '⊞' },
  { to: '/alerts', label: '警告通知', icon: '⚠' },
]

export default function Layout() {
  const navigate = useNavigate()
  const [menuOpen, setMenuOpen] = useState(false)
  const [serverVer, setServerVer] = useState('')
  const [theme, setTheme] = useState<Theme>(() => {
    return (localStorage.getItem('hawkeye-theme') as Theme) || 'cosmic'
  })

  useEffect(() => {
    versionInfo.get().then(r => setServerVer(r.version)).catch(() => {})
  }, [])

  useEffect(() => {
    document.documentElement.setAttribute('data-theme', theme)
    localStorage.setItem('hawkeye-theme', theme)
  }, [theme])

  const handleLogout = () => {
    localStorage.removeItem('auth_token')
    localStorage.removeItem('username')
    navigate('/')
    window.location.reload()
  }

  const handleNav = () => {
    setMenuOpen(false)
  }

  return (
    <div className="flex h-screen overflow-hidden">
      {/* Mobile overlay */}
      {menuOpen && (
        <div className="fixed inset-0 bg-black/50 z-40 md:hidden" onClick={() => setMenuOpen(false)} />
      )}

      {/* Sidebar */}
      <aside className={`
        fixed md:static inset-y-0 left-0 z-50
        w-56 bg-[#12122a]/90 backdrop-blur-xl flex flex-col shrink-0 border-r border-purple-500/10
        transform transition-transform duration-200 ease-in-out
        ${menuOpen ? 'translate-x-0' : '-translate-x-full'} md:translate-x-0
      `}>
        <div className="flex items-center justify-between px-5 py-4 border-b border-purple-500/10">
          <div>
            <h1 className="text-lg font-bold gradient-text">Hawkeye</h1>
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
                `flex items-center gap-3 px-3 py-2.5 rounded-lg text-sm transition-colors ${
                  isActive ? 'bg-accent/15 text-accent shadow-glow-sm' : 'text-gray-400 hover:text-gray-200 hover:bg-bg-hover'
                }`
              }
            >
              <span className="text-base">{item.icon}</span>
              {item.label}
            </NavLink>
          ))}
        </nav>
        <div className="px-5 py-3 border-t border-purple-500/10 space-y-2">
          <div className="flex items-center gap-2">
            {themes.map((t) => (
              <button
                key={t.key}
                onClick={() => setTheme(t.key)}
                title={t.label}
                className={`w-5 h-5 rounded-full border-2 transition-all ${
                  theme === t.key ? 'border-white scale-110 shadow-glow-sm' : 'border-transparent opacity-50 hover:opacity-80'
                }`}
                style={{ background: `linear-gradient(135deg, ${t.from}, ${t.to})` }}
              />
            ))}
          </div>
          <button
            onClick={handleLogout}
            className="w-full text-left text-xs text-gray-500 hover:text-red-400 transition-colors"
          >
            &larr; 退出
          </button>
          <span className="text-sm text-gray-600 block">ver.{serverVer || '...'}</span>
        </div>
      </aside>

      {/* Main Content */}
      <div className="flex-1 flex flex-col overflow-hidden">
        {/* Mobile header bar */}
        <header className="md:hidden flex items-center gap-3 px-4 py-3 bg-[#12122a] border-b border-purple-500/10 shrink-0">
          <button onClick={() => setMenuOpen(true)} className="text-gray-400 hover:text-gray-200 p-1">
            <svg className="w-6 h-6" fill="none" stroke="currentColor" viewBox="0 0 24 24">
              <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M4 6h16M4 12h16M4 18h16" />
            </svg>
          </button>
          <span className="text-sm font-semibold text-accent">Hawkeye</span>
        </header>
        <main className="flex-1 overflow-auto" style={{
          background: `
            linear-gradient(135deg, rgba(139, 92, 246, 0.3) 0%, rgba(139, 92, 246, 0.2) 12%, rgba(15, 10, 30, 0.85) 25%, rgba(10, 10, 26, 0.9) 50%, rgba(232, 121, 36, 0.2) 80%, rgba(245, 178, 50, 0.3) 100%)
          `,
          backgroundColor: '#0a0a1a'
        }}>
          <Outlet />
        </main>
      </div>
    </div>
  )
}
