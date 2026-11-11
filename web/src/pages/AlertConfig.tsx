import { useEffect, useState } from 'react'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { alertEvents, alertV2, settings, database } from '../api/client'
import { toCSTFull } from '../utils'
import type { AlertEvent } from '../types'
import AlertEntry from '../components/AlertEntry'

type Tab = 'machine' | 'web' | 'events' | 'notify'

export default function AlertConfig() {
  const queryClient = useQueryClient()
  const [tab, setTab] = useState<Tab>('machine')
  const [webhookUrl, setWebhookUrl] = useState('')
  const [testStatus, setTestStatus] = useState<'idle' | 'sending' | 'success' | 'error'>('idle')
  const [testMsg, setTestMsg] = useState('')

  const { data: events = [] } = useQuery({
    queryKey: ['alert-events'],
    queryFn: () => alertEvents.list(false, 100),
    refetchInterval: 15000,
  })
  const { data: settingsData = [] } = useQuery({ queryKey: ['settings'], queryFn: settings.list })
  const { data: health } = useQuery({
    queryKey: ['alert-health'],
    queryFn: alertV2.health,
    refetchInterval: 30000,
  })
  const { data: dbInfo } = useQuery({
    queryKey: ['database-info'],
    queryFn: database.info,
    refetchInterval: 30000,
  })

  useEffect(() => {
    const s = settingsData.find((s) => s.key === 'default_wechat_webhook')
    if (s && s.value) setWebhookUrl(s.value)
  }, [settingsData])

  const saveWebhook = useMutation({
    mutationFn: (url: string) => settings.update({ default_wechat_webhook: url }),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['settings'] })
      setTestStatus('idle')
    },
  })
  const clearEvents = useMutation({
    mutationFn: () => alertEvents.clear(),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: ['alert-events'] }),
  })
  const purgeDb = useMutation({
    mutationFn: () => database.purge(),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['alert-events'] })
      queryClient.invalidateQueries({ queryKey: ['database-info'] })
    },
  })

  const handleTestWebhook = async () => {
    if (!webhookUrl.trim()) return
    setTestStatus('sending')
    setTestMsg('')
    try {
      await alertEvents.testWebhook(webhookUrl.trim())
      setTestStatus('success')
      setTestMsg('测试通知已发送')
    } catch (e: any) {
      setTestStatus('error')
      setTestMsg(e.message || '发送失败')
    }
  }

  const currentWebhook = settingsData.find((s) => s.key === 'default_wechat_webhook')?.value || ''

  return (
    <div className="py-4 px-2 md:py-6 md:px-3 max-w-[1920px] mx-auto">
      <div className="flex items-center justify-between mb-4 md:mb-6">
        <div>
          <h2 className="page-title">警告配置</h2>
          <p className="page-sub">机器和网站分开管，每个目标一行独立阈值</p>
        </div>
      </div>

      <div className="flex gap-1 mb-6 bg-bg-card/70 rounded-lg p-1 w-fit">
        {([
          { key: 'machine', label: '🖥 机器监控' },
          { key: 'web', label: '🌐 网页监控' },
          { key: 'events', label: '警告历史' },
          { key: 'notify', label: '通知方式' },
        ] as const).map((t) => (
          <button
            key={t.key}
            onClick={() => setTab(t.key as Tab)}
            className={`tab-pill ${tab === t.key ? 'tab-pill-active' : ''}`}
          >
            {t.label}
          </button>
        ))}
      </div>

      {tab === 'machine' && <AlertEntry cls="agent" />}
      {tab === 'web' && <AlertEntry cls="probe" />}

      {tab === 'events' && (
        <>
          <div className="flex justify-end mb-3">
            <button
              onClick={() => { if (confirm('确认清除所有警告历史？')) clearEvents.mutate() }}
              disabled={clearEvents.isPending || events.length === 0}
              className="px-3 py-1.5 rounded-lg text-xs text-err border border-err/40 hover:bg-err/10 transition-colors disabled:opacity-50"
            >
              {clearEvents.isPending ? '清除中...' : '清除历史'}
            </button>
          </div>
          <div className="bg-bg-card/70 rounded-xl overflow-hidden">
            <table className="w-full text-sm">
              <thead>
                <tr className="border-b text-left text-fg-2">
                  <th className="px-4 py-3">时间</th>
                  <th className="px-4 py-3">级别</th>
                  <th className="px-4 py-3">消息</th>
                  <th className="px-4 py-3">值</th>
                  <th className="px-4 py-3">状态</th>
                </tr>
              </thead>
              <tbody>
                {events.map((event: AlertEvent) => (
                  <tr key={event.id}>
                    <td className="px-4 py-2.5 text-fg-1 text-xs">{toCSTFull(event.fired_at)}</td>
                    <td className="px-4 py-2.5">
                      <span className={`text-xs px-2 py-0.5 rounded ${event.severity === 'critical' ? 'bg-err/10 text-err' : 'bg-warn/10 text-warn'}`}>
                        {event.severity === 'critical' ? '严重' : '警告'}
                      </span>
                    </td>
                    <td className="px-4 py-2.5 text-fg-1">{event.message}</td>
                    <td className="px-4 py-2.5 text-fg-1">{event.value !== null ? event.value.toFixed(1) : '-'}</td>
                    <td className="px-4 py-2.5">
                      {event.resolved_at
                        ? <span className="text-ok text-xs">已恢复</span>
                        : <span className="text-err text-xs">警告中</span>}
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
            {events.length === 0 && <div className="py-12 text-center text-fg-3">暂无警告记录</div>}
          </div>
        </>
      )}

      {tab === 'notify' && (
        <div className="bg-bg-card/70 rounded-xl p-6 space-y-5">
          <div>
            <h3 className="text-lg font-semibold mb-1">企业微信通知</h3>
            <p className="text-fg-2 text-sm">
              只用一个群机器人 webhook。所有规则都走这里，单条规则也可以单独填地址覆盖。
            </p>
          </div>

          <div>
            <label className="text-xs text-fg-3 block mb-1">当前状态</label>
            <div className="flex items-center gap-2">
              <span className={`inline-block w-2 h-2 rounded-full ${currentWebhook ? 'bg-ok' : 'bg-gray-500'}`} />
              <span className="text-sm text-fg-1">{currentWebhook ? '已配置' : '未配置'}</span>
            </div>
          </div>

          <div>
            <label className="text-xs text-fg-3 block mb-1">Webhook 地址</label>
            <input
              value={webhookUrl}
              onChange={(e) => setWebhookUrl(e.target.value)}
              placeholder="https://qyapi.weixin.qq.com/cgi-bin/webhook/send?key=..."
              className="w-full bg-bg-card/80 border border-white/10 text-fg-1 px-3 py-2 rounded-lg border border-white/10 text-sm text-fg-1 focus:border-accent outline-none"
            />
          </div>

          <div className="flex items-center gap-3">
            <button
              onClick={() => saveWebhook.mutate(webhookUrl.trim())}
              disabled={saveWebhook.isPending}
              className="px-4 py-2 bg-accent text-black rounded-lg text-sm font-medium hover:bg-accent/80 disabled:opacity-40"
            >
              {saveWebhook.isPending ? '保存中...' : '保存'}
            </button>
            <button
              onClick={handleTestWebhook}
              disabled={!webhookUrl.trim() || testStatus === 'sending'}
              className="px-4 py-2 rounded-lg text-sm border border-white/20 hover:bg-white/5 disabled:opacity-40"
            >
              {testStatus === 'sending' ? '发送中...' : '发送测试消息'}
            </button>
            {testStatus !== 'idle' && (
              <span className={`text-sm ${testStatus === 'success' ? 'text-ok' : testStatus === 'error' ? 'text-err' : 'text-fg-3'}`}>
                {testMsg}
              </span>
            )}
          </div>

          {health?.outbox && (health.outbox.failed > 0 || health.outbox.pending > 0) && (
            <div className="text-sm text-warn">
              通知队列：待重试 {health.outbox.pending || 0} 条，永久失败 {health.outbox.failed || 0} 条
              （webhook 地址失效时会出现）
            </div>
          )}

          <div className="border-t border-white/10 pt-4">
            <div className="flex items-center justify-between">
              <div>
                <div className="text-sm text-fg-1">数据库</div>
                <div className="text-xs text-fg-3 mt-0.5">
                  {dbInfo ? `${(dbInfo.size_bytes / 1024 / 1024).toFixed(1)} MB` : '—'}
                </div>
              </div>
              <button
                onClick={() => { if (confirm('确认清除数据库文件？此操作会清空历史数据')) purgeDb.mutate() }}
                disabled={purgeDb.isPending}
                className="px-3 py-1.5 rounded-lg text-xs text-err border border-err/40 hover:bg-err/10 disabled:opacity-50"
              >
                {purgeDb.isPending ? '清除中...' : '清除数据库文件'}
              </button>
            </div>
          </div>
        </div>
      )}
    </div>
  )
}
