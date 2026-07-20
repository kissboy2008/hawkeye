import { useEffect, useRef } from 'react'
import type { QueryClient } from '@tanstack/react-query'

// ── Module-level singleton ────────────────────────────────────────────
let ws: WebSocket | null = null
let reconnectTimer: ReturnType<typeof setTimeout> | undefined
let refCount = 0

function connect(queryClient: QueryClient) {
  const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:'
  const socket = new WebSocket(`${protocol}//${window.location.host}/ws`)

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
        // Invalidate probes so UI picks up the latest
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
      reconnectTimer = setTimeout(() => connect(queryClient), 5000)
    }
  }

  socket.onerror = () => {
    socket.close()
  }

  ws = socket
}

function disconnect() {
  clearTimeout(reconnectTimer)
  ws?.close()
  ws = null
}

// ── Hook ──────────────────────────────────────────────────────────────

export function useWebSocket(queryClient: QueryClient) {
  const qcRef = useRef(queryClient)
  qcRef.current = queryClient

  useEffect(() => {
    refCount++
    if (!ws) {
      connect(qcRef.current)
    }
    return () => {
      refCount--
      if (refCount <= 0) {
        refCount = 0
        disconnect()
      }
    }
  }, [])
}
