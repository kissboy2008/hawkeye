import { useEffect, useRef } from 'react'
import type { QueryClient } from '@tanstack/react-query'

// ── Module-level singleton ────────────────────────────────────────────
let ws: WebSocket | null = null
let reconnectTimer: ReturnType<typeof setTimeout> | undefined
let refCount = 0
// 指数退避：1s → 2s → 4s … 上限 30s，永不放弃重连。
// 服务端重启/网络抖动时旧实现退避 3 次就永久停止，页面从此只能靠 REST 轮询。
let failCount = 0
const BASE_DELAY = 1000
const MAX_DELAY = 30000

function nextDelay() {
  return Math.min(BASE_DELAY * 2 ** failCount, MAX_DELAY)
}

function scheduleReconnect(queryClient: QueryClient) {
  if (refCount <= 0) return
  clearTimeout(reconnectTimer)
  reconnectTimer = setTimeout(() => connect(queryClient), nextDelay())
}

function connect(queryClient: QueryClient) {
  if (refCount <= 0) return
  const token = localStorage.getItem('auth_token')
  if (!token) return

  const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:'
  const url = `${protocol}//${window.location.host}/ws?token=${encodeURIComponent(token)}`
  const socket = new WebSocket(url)

  socket.onopen = () => {
    failCount = 0 // 连接正常，重置退避
  }

  socket.onmessage = (event) => {
    try {
      const msg = JSON.parse(event.data)
      if (msg.type === 'metrics' && msg.agent_id != null) {
        // Transform WS payload → REST API cache shape
        const d = msg.data || {}
        queryClient.setQueryData(['latest', msg.agent_id], {
          agent_id: msg.agent_id,
          timestamp: d.timestamp || new Date().toISOString(),
          metrics: {
            cpu: d.cpu ? JSON.stringify(d.cpu) : '',
            memory: d.memory ? JSON.stringify(d.memory) : '',
            uptime: JSON.stringify({ uptime_seconds: d.uptime_seconds ?? 0 }),
          },
        })
      }
      if (msg.type === 'probe_result') {
        queryClient.invalidateQueries({ queryKey: ['probes'] })
      }
      if (msg.type === 'alert') {
        queryClient.invalidateQueries({ queryKey: ['active-alerts'] })
      }
    } catch { /* ignore malformed messages */ }
  }

  socket.onclose = () => {
    ws = null
    if (refCount > 0) {
      scheduleReconnect(queryClient) // 用当前 failCount 计算退避
      failCount++
    }
  }

  socket.onerror = () => {
    // close 事件随后会到达并触发重连
    socket.close()
  }

  ws = socket
}

function disconnect() {
  clearTimeout(reconnectTimer)
  failCount = 0
  ws?.close()
  ws = null
}

// 系统网络恢复时立刻重连，不用等退避上限
let lastQueryClient: QueryClient | undefined

function handleOnline() {
  if (refCount > 0 && !ws) {
    failCount = 0
    clearTimeout(reconnectTimer)
    if (lastQueryClient) connect(lastQueryClient)
  }
}

// ── Hook ──────────────────────────────────────────────────────────────

export function useWebSocket(queryClient: QueryClient) {
  const qcRef = useRef(queryClient)
  qcRef.current = queryClient
  lastQueryClient = queryClient

  useEffect(() => {
    refCount++
    window.addEventListener('online', handleOnline)
    if (!ws) {
      connect(qcRef.current)
    }
    return () => {
      window.removeEventListener('online', handleOnline)
      refCount--
      if (refCount <= 0) {
        refCount = 0
        disconnect()
      }
    }
  }, [])
}
