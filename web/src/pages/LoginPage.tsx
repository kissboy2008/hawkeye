import { useState, useEffect } from 'react'
import { auth } from '../api/client'

export default function LoginPage({ onLogin }: { onLogin: () => void }) {
  const [username, setUsername] = useState('')
  const [password, setPassword] = useState('')
  const [error, setError] = useState('')
  const [loading, setLoading] = useState(false)
  const [needRegister, setNeedRegister] = useState(false)
  const [isRegister] = useState(false)

  useEffect(() => {
    auth.check().then(r => setNeedRegister(r.need_register)).catch(() => {})
  }, [])

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault()
    setError('')
    if (!username.trim() || !password) { setError('请填写账户和密码'); return }

    setLoading(true)
    try {
      let result
      if (isRegister || needRegister) {
        result = await auth.register(username.trim(), password)
      } else {
        result = await auth.login(username.trim(), password)
      }
      localStorage.setItem('auth_token', result.token)
      localStorage.setItem('username', result.username)
      document.cookie = `auth_token=${result.token}; path=/; SameSite=Lax`
      onLogin()
    } catch (err: any) {
      setError(err.message || '操作失败')
    } finally {
      setLoading(false)
    }
  }

  return (
    <div style={{
      display: 'flex', alignItems: 'center', justifyContent: 'center',
      minHeight: '100vh', background: '#06060e', position: 'relative', overflow: 'hidden',
    }}>
      {/* Corner decorations */}
      <div style={{ position: 'absolute', top: 20, left: 40, fontSize: 10, color: '#7c3aed20', letterSpacing: 3 }}>SECTOR-7G</div>
      <div style={{ position: 'absolute', bottom: 20, right: 40, fontSize: 10, color: '#7c3aed20', letterSpacing: 3 }}>NODE::ACTIVE</div>
      <div style={{ position: 'absolute', top: '50%', left: 20, width: 1, height: 60, background: '#7c3aed22', transform: 'translateY(-50%)' }} />
      <div style={{ position: 'absolute', top: '50%', right: 20, width: 1, height: 60, background: '#7c3aed22', transform: 'translateY(-50%)' }} />

      {/* Login card */}
      <div style={{
        width: 'calc(100% - 2rem)', maxWidth: 340, padding: '36px 30px', border: '1px solid #7c3aed55',
        borderRadius: 16, background: '#0b0b18', zIndex: 1,
      }}>
        {/* Header */}
        <div style={{ textAlign: 'center', marginBottom: 32 }}>
          <div style={{
            width: 56, height: 56, margin: '0 auto 12px',
            border: '2px solid #7c3aed', borderRadius: 12,
            display: 'flex', alignItems: 'center', justifyContent: 'center',
          }}>
            <svg width="28" height="28" viewBox="0 0 24 24" fill="none" stroke="#a78bfa" strokeWidth="2">
              <path d="M12 2L2 7l10 5 10-5-10-5zM2 17l10 5 10-5M2 12l10 5 10-5"/>
            </svg>
          </div>
          <div style={{ fontSize: 22, fontWeight: 700, color: '#e2e8f0' }}>Hawkeye</div>
          <div style={{ fontSize: 12, color: '#7c3aed66', marginTop: 4, letterSpacing: 3 }}>MONITOR CONTROL</div>
        </div>

        <form onSubmit={handleSubmit}>
          {/* Username */}
          <div style={{ marginBottom: 16 }}>
            <div style={{ fontSize: 11, color: '#8b5cf6', marginBottom: 6 }}>账户</div>
            <input
              style={{
                width: '100%', padding: '11px 14px', background: '#0f0f23',
                border: '1px solid #7c3aed33', borderRadius: 8, color: '#c4b5fd',
                fontSize: 14, outline: 'none', boxSizing: 'border-box',
              }}
              value={username}
              onChange={(e) => setUsername(e.target.value)}
              placeholder="admin"
              autoFocus
            />
          </div>

          {/* Password */}
          <div style={{ marginBottom: 24 }}>
            <div style={{ fontSize: 11, color: '#8b5cf6', marginBottom: 6 }}>密码</div>
            <input
              type="password"
              style={{
                width: '100%', padding: '11px 14px', background: '#0f0f23',
                border: '1px solid #7c3aed33', borderRadius: 8, color: '#c4b5fd',
                fontSize: 14, outline: 'none', boxSizing: 'border-box',
              }}
              value={password}
              onChange={(e) => setPassword(e.target.value)}
              placeholder="******"
            />
          </div>

          {/* Error */}
          {error && (
            <div style={{
              fontSize: 12, color: '#ef4444', marginBottom: 16,
              padding: '8px 12px', background: '#1a0a0a', borderRadius: 6,
              border: '1px solid #ef444433',
            }}>
              {error}
            </div>
          )}

          {/* Submit */}
          <button
            type="submit"
            disabled={loading}
            style={{
              width: '100%', padding: '11px 0', background: '#7c3aed',
              border: 'none', borderRadius: 8, color: '#fff', fontSize: 14,
              fontWeight: 600, cursor: loading ? 'wait' : 'pointer',
              opacity: loading ? 0.7 : 1,
            }}
          >
            {loading ? '验证中...' : (isRegister || needRegister) ? '注册' : '登入'}
          </button>

          {/* Registration hint */}
          {needRegister && !isRegister && (
            <div style={{
              marginTop: 20, textAlign: 'center', fontSize: 12, color: '#7c3aedaa',
            }}>
              首次使用，请先注册
            </div>
          )}
        </form>

        {/* Dots */}
        <div style={{ display: 'flex', justifyContent: 'center', gap: 24, marginTop: 24 }}>
          <div style={{ width: 6, height: 6, background: '#7c3aed', borderRadius: '50%', opacity: 0.3 }} />
          <div style={{ width: 6, height: 6, background: '#7c3aed', borderRadius: '50%' }} />
          <div style={{ width: 6, height: 6, background: '#7c3aed', borderRadius: '50%', opacity: 0.3 }} />
        </div>
      </div>
    </div>
  )
}
