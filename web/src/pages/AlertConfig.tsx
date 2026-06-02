import { useState, useEffect } from 'react'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { alertRules, alertEvents, settings, database } from '../api/client'
import type { AlertRule, AlertEvent } from '../types'

const OPERATORS = [
  { value: 'gt', label: '>' },
  { value: 'lt', label: '<' },
  { value: 'gte', label: '>=' },
  { value: 'lte', label: '<=' },
  { value: 'eq', label: '=' },
  { value: 'neq', label: '!=' },
]

const METRIC_TYPES = [
  { value: 'cpu', label: 'CPU 使用率 (%)' },
  { value: 'memory', label: '内存使用率 (%)' },
  { value: 'probe_latency', label: '探测延迟 (ms)' },
  { value: 'probe_status', label: '探测状态 (0/1)' },
]

export default function AlertConfig() {
  const queryClient = useQueryClient()
  const [tab, setTab] = useState<'rules' | 'events' | 'notify'>('rules')
  const [editing, setEditing] = useState<Partial<AlertRule> | null>(null)
  const [webhookUrl, setWebhookUrl] = useState('')
  const [testStatus, setTestStatus] = useState<'idle' | 'sending' | 'success' | 'error'>('idle')
  const [testMsg, setTestMsg] = useState('')

  const { data: rules = [] } = useQuery({ queryKey: ['alert-rules'], queryFn: alertRules.list })
  const { data: events = [] } = useQuery({
    queryKey: ['alert-events'],
    queryFn: () => alertEvents.list(false, 100),
    refetchInterval: 15000,
  })
  const { data: settingsData = [] } = useQuery({ queryKey: ['settings'], queryFn: settings.list })

  // Sync webhook URL from settings
  useEffect(() => {
    const s = settingsData.find((s) => s.key === 'default_wechat_webhook')
    if (s && s.value) setWebhookUrl(s.value)
  }, [settingsData])

  const create = useMutation({
    mutationFn: (data: Partial<AlertRule>) => alertRules.create(data),
    onSuccess: () => { queryClient.invalidateQueries({ queryKey: ['alert-rules'] }); setEditing(null) },
  })
  const update = useMutation({
    mutationFn: ({ id, ...data }: Partial<AlertRule> & { id: number }) => alertRules.update(id, data),
    onSuccess: () => { queryClient.invalidateQueries({ queryKey: ['alert-rules'] }); setEditing(null) },
  })
  const remove = useMutation({
    mutationFn: (id: number) => alertRules.delete(id),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: ['alert-rules'] }),
  })
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
  const { data: dbInfo } = useQuery({
    queryKey: ['database-info'],
    queryFn: database.info,
    refetchInterval: 30000,
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

  const handleSaveWebhook = () => {
    saveWebhook.mutate(webhookUrl.trim())
  }

  const handleSubmit = (e: React.FormEvent) => {
    e.preventDefault()
    if (!editing) return
    const data = {
      name: editing.name!,
      description: editing.description || '',
      scope_type: editing.scope_type || 'agent',
      scope_id: editing.scope_id || null,
      metric_type: editing.metric_type || 'cpu',
      operator: editing.operator || 'gt',
      threshold: editing.threshold || 0,
      duration_s: editing.duration_s || 0,
      enabled: editing.enabled ?? true,
      cooldown_s: editing.cooldown_s || 300,
      repeat_enabled: editing.repeat_enabled || false,
      repeat_interval_s: editing.repeat_interval_s || 300,
    }
    if (editing.id) {
      update.mutate({ id: editing.id, ...data })
    } else {
      create.mutate(data)
    }
  }

  const opLabel = (op: string) => OPERATORS.find((o) => o.value === op)?.label || op
  const metricLabel = (m: string) => METRIC_TYPES.find((t) => t.value === m)?.label || m

  const currentWebhook = settingsData.find((s) => s.key === 'default_wechat_webhook')?.value || ''

  return (
    <div className="p-4 md:p-6 max-w-[1800px] mx-auto">
      <div className="flex items-center justify-between mb-4 md:mb-6">
        <div>
          <h2 className="text-xl md:text-2xl font-bold mb-1">警告配置</h2>
          <p className="text-gray-500 text-sm">管理警告规则与通知</p>
        </div>
      </div>

      {/* Tabs */}
      <div className="flex gap-1 mb-6 bg-bg-card rounded-lg p-1 w-fit">
        {([
          { key: 'rules', label: '警告规则' },
          { key: 'events', label: '警告历史' },
          { key: 'notify', label: '通知方式' },
        ] as const).map((t) => (
          <button
            key={t.key}
            onClick={() => setTab(t.key)}
            className={`px-4 py-2 rounded-md text-sm transition-colors ${
              tab === t.key ? 'bg-accent/10 text-accent' : 'text-gray-400 hover:text-gray-200'
            }`}
          >
            {t.label}
          </button>
        ))}
      </div>

      {tab === 'rules' && (
        <>
          <div className="flex justify-end mb-4">
            <button
              onClick={() => setEditing({ name: '', scope_type: 'agent', metric_type: 'cpu', operator: 'gt', threshold: 90, duration_s: 60, cooldown_s: 300, repeat_enabled: false, repeat_interval_s: 300 })}
              className="px-4 py-2 bg-accent text-black rounded-lg text-sm font-medium hover:bg-accent/80"
            >
              + 新建规则
            </button>
          </div>

          <div className="space-y-2">
            {rules.map((rule) => (
              <div key={rule.id} className="bg-bg-card rounded-xl px-4 py-3 flex items-center justify-between">
                <div>
                  <div className="flex items-center gap-2">
                    <span className={`text-xs px-2 py-0.5 rounded ${rule.enabled ? 'bg-ok/10 text-ok' : 'bg-gray-500/10 text-gray-500'}`}>
                      {rule.enabled ? '启用' : '禁用'}
                    </span>
                    <span className="font-medium">{rule.name}</span>
                  </div>
                  <div className="text-xs text-gray-400 mt-1">
                    {rule.scope_type === 'probe' ? '探测' : '机器'} |
                    {metricLabel(rule.metric_type)} {opLabel(rule.operator)} {rule.threshold}
                    {rule.duration_s > 0 && ` | 持续 ${rule.duration_s}s`}
                    {rule.repeat_enabled && ` | 每${Math.round(rule.repeat_interval_s / 60)}分钟重复`}
                  </div>
                </div>
                <div className="flex gap-2">
                  <button onClick={() => setEditing(rule)} className="text-accent hover:underline text-xs">编辑</button>
                  <button
                    onClick={() => {
                      const next = { ...rule, enabled: !rule.enabled }
                      update.mutate(next)
                    }}
                    className="text-warn hover:underline text-xs"
                  >
                    {rule.enabled ? '禁用' : '启用'}
                  </button>
                  <button onClick={() => { if (confirm('确认删除?')) remove.mutate(rule.id) }} className="text-err hover:underline text-xs">删除</button>
                </div>
              </div>
            ))}
            {rules.length === 0 && <div className="py-12 text-center text-gray-500">暂无警告规则</div>}
          </div>
        </>
      )}

      {tab === 'events' && (
        <>
          <div className="flex justify-end mb-3">
            <button
              onClick={() => { if (confirm('确认清除所有警告历史？')) clearEvents.mutate() }}
              disabled={clearEvents.isPending || events.length === 0}
              className="px-3 py-1.5 rounded-lg text-xs text-err border border-err/40 hover:bg-err/10 transition-colors disabled:opacity-50 flex items-center gap-1.5"
            >
              <svg className="w-3.5 h-3.5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M19 7l-.867 12.142A2 2 0 0116.138 21H7.862a2 2 0 01-1.995-1.858L5 7m5 4v6m4-6v6m1-10V4a1 1 0 00-1-1h-4a1 1 0 00-1 1v3M4 7h16" />
              </svg>
              {clearEvents.isPending ? '清除中...' : '清除历史'}
            </button>
          </div>
          <div className="bg-bg-card rounded-xl overflow-hidden">
          <table className="w-full text-sm">
            <thead>
              <tr className="border-b border-purple-500/10 text-left text-gray-500">
                <th className="px-4 py-3">时间</th>
                <th className="px-4 py-3">级别</th>
                <th className="px-4 py-3">消息</th>
                <th className="px-4 py-3">值</th>
                <th className="px-4 py-3">状态</th>
              </tr>
            </thead>
            <tbody>
              {events.map((event: AlertEvent) => (
                <tr key={event.id} className="border-b border-purple-500/10/50">
                  <td className="px-4 py-2.5 text-gray-400 text-xs">
                    {new Date(event.fired_at).toLocaleString('zh-CN')}
                  </td>
                  <td className="px-4 py-2.5">
                    <span className={`text-xs px-2 py-0.5 rounded ${event.severity === 'critical' ? 'bg-err/10 text-err' : 'bg-warn/10 text-warn'}`}>
                      {event.severity === 'critical' ? '严重' : '警告'}
                    </span>
                  </td>
                  <td className="px-4 py-2.5 text-gray-300">{event.message.replace(/> /g, ' ').trim()}</td>
                  <td className="px-4 py-2.5 text-gray-400">{event.value !== null ? event.value : '-'}</td>
                  <td className="px-4 py-2.5">
                    {event.resolved_at
                      ? <span className="text-ok text-xs">已恢复</span>
                      : <span className="text-err text-xs">警告中</span>
                    }
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
          {events.length === 0 && <div className="py-12 text-center text-gray-500">暂无警告事件</div>}
        </div>

        {/* Database info */}
        <div className="mt-4 flex items-center justify-between bg-bg-card rounded-xl px-4 py-3">
          <div className="flex items-center gap-2 text-sm text-gray-400">
            <svg className="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
              <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M4 7v10c0 2.21 3.582 4 8 4s8-1.79 8-4V7M4 7c0 2.21 3.582 4 8 4s8-1.79 8-4M4 7c0-2.21 3.582-4 8-4s8 1.79 8 4" />
            </svg>
            <span>数据库文件大小</span>
            <span className="font-mono text-gray-200">
              {dbInfo ? (dbInfo.size_bytes >= 1048576 ? `${(dbInfo.size_bytes / 1048576).toFixed(1)} MB` : `${(dbInfo.size_bytes / 1024).toFixed(0)} KB`) : '—'}
            </span>
          </div>
          <button
            onClick={() => { if (confirm('确认清除数据库文件？此操作将删除所有监控数据（指标、探测结果、警告事件），不可撤销。')) purgeDb.mutate() }}
            disabled={purgeDb.isPending}
            className="px-3 py-1.5 rounded-lg text-xs text-err border border-err/40 hover:bg-err/10 transition-colors disabled:opacity-50 flex items-center gap-1.5"
          >
            <svg className="w-3.5 h-3.5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
              <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M19 7l-.867 12.142A2 2 0 0116.138 21H7.862a2 2 0 01-1.995-1.858L5 7m5 4v6m4-6v6m1-10V4a1 1 0 00-1-1h-4a1 1 0 00-1 1v3M4 7h16" />
            </svg>
            {purgeDb.isPending ? '清除中...' : '清除数据库文件'}
          </button>
        </div>
        </>
      )}

      {tab === 'notify' && (
        <div className="bg-bg-card rounded-xl p-6 space-y-5">
          <div>
            <h3 className="text-lg font-semibold mb-1">企业微信通知</h3>
            <p className="text-gray-500 text-sm">
              配置企业微信机器人 Webhook 地址，警告触发时将通过该地址发送通知。
              所有未单独配置 Webhook 的警告规则均使用此全局地址。
            </p>
          </div>

          <div>
            <label className="text-xs text-gray-400 block mb-1">当前状态</label>
            <div className="flex items-center gap-2">
              <span className={`inline-block w-2 h-2 rounded-full ${currentWebhook ? 'bg-ok' : 'bg-gray-500'}`} />
              <span className="text-sm text-gray-300">
                {currentWebhook ? '已配置' : '未配置'}
              </span>
            </div>
          </div>

          <div>
            <label className="text-xs text-gray-400 block mb-1">Webhook 地址</label>
            <input
              className="w-full bg-bg px-3 py-2.5 rounded-lg border border-purple-500/10 text-sm focus:border-accent outline-none font-mono"
              placeholder="https://qyapi.weixin.qq.com/cgi-bin/webhook/send?key=..."
              value={webhookUrl}
              onChange={(e) => setWebhookUrl(e.target.value)}
            />
          </div>

          <div className="flex gap-3">
            <button
              onClick={handleSaveWebhook}
              disabled={saveWebhook.isPending}
              className="px-5 py-2 bg-accent text-black rounded-lg text-sm font-medium hover:bg-accent/80 disabled:opacity-50"
            >
              {saveWebhook.isPending ? '保存中...' : '保存'}
            </button>
            <button
              onClick={handleTestWebhook}
              disabled={testStatus === 'sending' || !webhookUrl.trim()}
              className="px-5 py-2 bg-bg-hover text-gray-200 rounded-lg text-sm hover:bg-gray-700 disabled:opacity-50"
            >
              {testStatus === 'sending' ? '发送中...' : '测试通知'}
            </button>
          </div>

          {testMsg && (
            <div className={`text-sm px-3 py-2 rounded-lg ${
              testStatus === 'success'
                ? 'bg-ok/10 text-ok'
                : testStatus === 'error'
                  ? 'bg-err/10 text-err'
                  : 'bg-gray-700 text-gray-300'
            }`}>
              {testMsg}
            </div>
          )}
        </div>
      )}

      {/* Modal */}
      {editing && (
        <div className="fixed inset-0 bg-black/50 flex items-center justify-center z-50">
          <div className="bg-bg-card rounded-xl p-4 md:p-6 w-[calc(100%-2rem)] max-w-md mx-4 border border-purple-500/10">
            <h3 className="text-lg font-semibold mb-4">{editing.id ? '编辑规则' : '新建规则'}</h3>
            <form onSubmit={handleSubmit} className="space-y-3">
              <div>
                <label className="text-xs text-gray-400 block mb-1">规则名称</label>
                <input
                  className="w-full bg-bg px-3 py-2 rounded-lg border border-purple-500/10 text-sm focus:border-accent outline-none"
                  value={editing.name || ''} onChange={(e) => setEditing({ ...editing, name: e.target.value })} required
                />
              </div>
              <div>
                <label className="text-xs text-gray-400 block mb-1">描述</label>
                <input
                  className="w-full bg-bg px-3 py-2 rounded-lg border border-purple-500/10 text-sm focus:border-accent outline-none"
                  value={editing.description || ''} onChange={(e) => setEditing({ ...editing, description: e.target.value })}
                />
              </div>
              <div className="grid grid-cols-2 gap-3">
                <div>
                  <label className="text-xs text-gray-400 block mb-1">作用范围</label>
                  <select
                    className="w-full bg-bg px-3 py-2 rounded-lg border border-purple-500/10 text-sm focus:border-accent outline-none"
                    value={editing.scope_type || 'agent'} onChange={(e) => setEditing({ ...editing, scope_type: e.target.value as 'agent' | 'probe' })}
                  >
                    <option value="agent">机器</option>
                    <option value="probe">探测</option>
                  </select>
                </div>
                <div>
                  <label className="text-xs text-gray-400 block mb-1">指标类型</label>
                  <select
                    className="w-full bg-bg px-3 py-2 rounded-lg border border-purple-500/10 text-sm focus:border-accent outline-none"
                    value={editing.metric_type || 'cpu'} onChange={(e) => setEditing({ ...editing, metric_type: e.target.value })}
                  >
                    {METRIC_TYPES.map((t) => <option key={t.value} value={t.value}>{t.label}</option>)}
                  </select>
                </div>
              </div>
              <div className="grid grid-cols-3 gap-3">
                <div>
                  <label className="text-xs text-gray-400 block mb-1">运算符</label>
                  <select
                    className="w-full bg-bg px-3 py-2 rounded-lg border border-purple-500/10 text-sm focus:border-accent outline-none"
                    value={editing.operator || 'gt'} onChange={(e) => setEditing({ ...editing, operator: e.target.value })}
                  >
                    {OPERATORS.map((o) => <option key={o.value} value={o.value}>{o.label}</option>)}
                  </select>
                </div>
                <div>
                  <label className="text-xs text-gray-400 block mb-1">阈值</label>
                  <input
                    className="w-full bg-bg px-3 py-2 rounded-lg border border-purple-500/10 text-sm focus:border-accent outline-none"
                    type="number" step="any" value={editing.threshold || 0} onChange={(e) => setEditing({ ...editing, threshold: Number(e.target.value) })} required
                  />
                </div>
                <div>
                  <label className="text-xs text-gray-400 block mb-1">持续 (秒)</label>
                  <input
                    className="w-full bg-bg px-3 py-2 rounded-lg border border-purple-500/10 text-sm focus:border-accent outline-none"
                    type="number" value={editing.duration_s || 0} onChange={(e) => setEditing({ ...editing, duration_s: Number(e.target.value) })}
                  />
                </div>
              </div>
              <div>
                <label className="text-xs text-gray-400 block mb-1">冷却时间 (秒)</label>
                <input
                  className="w-full bg-bg px-3 py-2 rounded-lg border border-purple-500/10 text-sm focus:border-accent outline-none"
                  type="number" value={editing.cooldown_s || 300} onChange={(e) => setEditing({ ...editing, cooldown_s: Number(e.target.value) })}
                />
              </div>
              <div className="flex items-center gap-3">
                <label className="flex items-center gap-2 cursor-pointer">
                  <input
                    type="checkbox"
                    className="w-4 h-4 accent-accent rounded"
                    checked={editing.repeat_enabled || false}
                    onChange={(e) => setEditing({ ...editing, repeat_enabled: e.target.checked })}
                  />
                  <span className="text-sm text-gray-300">重复提醒</span>
                </label>
                {(editing.repeat_enabled) && (
                  <div className="flex items-center gap-1">
                    <span className="text-xs text-gray-400">每</span>
                    <input
                      className="w-16 bg-bg px-2 py-1.5 rounded-lg border border-purple-500/10 text-sm focus:border-accent outline-none text-center"
                      type="number" min="1" value={Math.round((editing.repeat_interval_s || 300) / 60)} onChange={(e) => setEditing({ ...editing, repeat_interval_s: Math.max(Number(e.target.value), 1) * 60 })}
                    />
                    <span className="text-xs text-gray-400">分钟</span>
                  </div>
                )}
              </div>
              <div className="flex gap-3 pt-2">
                <button type="submit" className="flex-1 bg-accent text-black rounded-lg py-2 text-sm font-medium">
                  {editing.id ? '保存' : '创建'}
                </button>
                <button type="button" onClick={() => setEditing(null)} className="flex-1 bg-bg-hover rounded-lg py-2 text-sm">取消</button>
              </div>
            </form>
          </div>
        </div>
      )}
    </div>
  )
}
