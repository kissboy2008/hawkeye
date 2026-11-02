import { useMemo, useState } from 'react'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { alertV2, agents as agentsApi, probes as probesApi } from '../api/client'
import type { AlertMetricDef, AlertRule, AlertRuleTarget } from '../types'

type Class = 'agent' | 'probe'

const STATUS_STYLE: Record<string, string> = {
  ok: 'text-ok',
  pending: 'text-warn',
  firing: 'text-err',
  silenced: 'text-white/60',
}
const STATUS_LABEL: Record<string, string> = {
  ok: '正常',
  pending: '待触发',
  firing: '告警中',
  silenced: '已静默',
}

/**
 * 机器监控 / 网页监控共用的规则面板。
 * 两个入口结构完全一致，但数据、指标、目标互不相通（后端按 target_class 隔离）。
 */
export default function AlertEntry({ cls }: { cls: Class }) {
  const qc = useQueryClient()
  const isAgent = cls === 'agent'

  const { data: rules = [] } = useQuery({
    queryKey: ['alert-v2-rules', cls],
    queryFn: () => alertV2.rules(cls),
    refetchInterval: 20000,
  })
  const { data: metricDefs = [] } = useQuery({
    queryKey: ['alert-v2-metrics', cls],
    queryFn: () => alertV2.metrics(cls),
  })
  const { data: agentList = [] } = useQuery({ queryKey: ['agents'], queryFn: agentsApi.list })
  const { data: probeList = [] } = useQuery({ queryKey: ['probes'], queryFn: probesApi.list })

  const [creating, setCreating] = useState(false)
  const [expanded, setExpanded] = useState<number | null>(null)

  const targets = useMemo(
    () => (isAgent ? agentList.map((a) => ({ id: a.id, name: a.name, sub: a.address }))
                   : probeList.map((p) => ({ id: p.id, name: p.name, sub: p.url }))),
    [isAgent, agentList, probeList],
  )

  return (
    <div>
      <div className="flex items-center justify-between mb-3">
        <p className="text-white text-sm">
          {isAgent
            ? '每台机器一行独立配置：离线多久、CPU 超过多少、持续几秒，各自单独设。'
            : '每个网站在一行独立配置：状态码、响应时间、证书剩余天数，各自单独设。'}
        </p>
        <button
          onClick={() => setCreating(true)}
          className="px-4 py-2 bg-accent text-black rounded-lg text-sm font-medium hover:bg-accent/80"
        >
          ＋ 新建规则
        </button>
      </div>

      {creating && (
        <CreateRuleForm
          cls={cls}
          metricDefs={metricDefs}
          targets={targets}
          onClose={() => setCreating(false)}
          onSaved={() => {
            setCreating(false)
            qc.invalidateQueries({ queryKey: ['alert-v2-rules', cls] })
          }}
        />
      )}

      <div className="space-y-3">
        {rules.map((rule) => (
          <RuleCard
            key={rule.id}
            rule={rule}
            cls={cls}
            metricDefs={metricDefs}
            allTargets={targets}
            expanded={expanded === rule.id}
            onToggle={() => setExpanded(expanded === rule.id ? null : rule.id)}
          />
        ))}
        {rules.length === 0 && !creating && (
          <div className="py-12 text-center text-white/70">
            还没有{isAgent ? '机器' : '网页'}告警规则，点右上角新建
          </div>
        )}
      </div>
    </div>
  )
}

function CreateRuleForm({
  cls,
  metricDefs,
  targets,
  onClose,
  onSaved,
}: {
  cls: Class
  metricDefs: AlertMetricDef[]
  targets: { id: number; name: string; sub: string }[]
  onClose: () => void
  onSaved: () => void
}) {
  const [name, setName] = useState('')
  const [metric, setMetric] = useState(metricDefs[0]?.key || '')
  const [threshold, setThreshold] = useState<number>(metricDefs[0]?.default_threshold ?? 0)
  const [durationS, setDurationS] = useState(0)
  const [cooldownS, setCooldownS] = useState(60)
  const [repeatEnabled, setRepeatEnabled] = useState(true)
  const [repeatIntervalS, setRepeatIntervalS] = useState(1800)
  const [selected, setSelected] = useState<number[]>([])

  const def = metricDefs.find((m) => m.key === metric)
  const availability = !!def?.availability

  // 切换指标时带出该指标的推荐阈值，省得每次自己猜
  const pickMetric = (key: string) => {
    setMetric(key)
    const d = metricDefs.find((m) => m.key === key)
    if (d) setThreshold(d.default_threshold)
  }

  const create = useMutation({
    mutationFn: () =>
      alertV2.createRule(cls, {
        name: name || def?.label || '未命名规则',
        metric_type: metric,
        operator: availability ? 'gte' : def?.default_operator || 'gt',
        threshold,
        duration_s: durationS,
        cooldown_s: cooldownS,
        repeat_enabled: repeatEnabled,
        repeat_interval_s: repeatIntervalS,
        targets: selected.map((id) => ({ target_type: cls, target_id: id })),
      }),
    onSuccess: onSaved,
    onError: (e: any) => alert(e?.message || '创建失败'),
  })

  const toggle = (id: number) =>
    setSelected((prev) => (prev.includes(id) ? prev.filter((x) => x !== id) : [...prev, id]))

  return (
    <div className="bg-bg-card/70 rounded-xl p-4 mb-4 border border-accent/30">
      <div className="flex items-center justify-between mb-3">
        <h3 className="font-medium">新建规则（{cls === 'agent' ? '机器监控' : '网页监控'}）</h3>
        <button onClick={onClose} className="text-white/60 hover:text-white text-sm">收起</button>
      </div>

      <div className="grid md:grid-cols-4 gap-3 mb-3">
        <label className="text-xs text-white/70">
          规则名称
          <input
            value={name}
            onChange={(e) => setName(e.target.value)}
            placeholder={def?.label || '例如 机器离线'}
            className="mt-1 w-full bg-bg-card/80 border border-white/10 text-white rounded px-2 py-1.5 text-sm text-white"
          />
        </label>
        <label className="text-xs text-white/70">
          监控指标
          <select
            value={metric}
            onChange={(e) => pickMetric(e.target.value)}
            className="mt-1 w-full bg-bg-card/80 border border-white/10 text-white rounded px-2 py-1.5 text-sm text-white"
          >
            {metricDefs.map((m) => (
              <option key={m.key} value={m.key}>{m.label}</option>
            ))}
          </select>
        </label>
        <label className="text-xs text-white/70">
          {availability ? '离线多久才算（分钟）' : `阈值（${def?.unit || ''}）`}
          <input
            type="number"
            value={threshold}
            onChange={(e) => setThreshold(Number(e.target.value))}
            className="mt-1 w-full bg-bg-card/80 border border-white/10 text-white rounded px-2 py-1.5 text-sm text-white"
          />
        </label>
        <label className="text-xs text-white/70">
          持续多久才告警（秒）
          <input
            type="number"
            value={durationS}
            onChange={(e) => setDurationS(Number(e.target.value))}
            className="mt-1 w-full bg-bg-card/80 border border-white/10 text-white rounded px-2 py-1.5 text-sm text-white"
          />
        </label>
      </div>

      <div className="grid md:grid-cols-3 gap-3 mb-3">
        <label className="text-xs text-white/70">
          恢复后防抖（秒）
          <input
            type="number"
            value={cooldownS}
            onChange={(e) => setCooldownS(Number(e.target.value))}
            className="mt-1 w-full bg-bg-card/80 border border-white/10 text-white rounded px-2 py-1.5 text-sm text-white"
          />
        </label>
        <label className="text-xs text-white/70 flex items-center gap-2 mt-4">
          <input type="checkbox" checked={repeatEnabled} onChange={(e) => setRepeatEnabled(e.target.checked)} />
          未恢复时重复提醒
        </label>
        <label className="text-xs text-white/70">
          重复间隔（秒）
          <input
            type="number"
            value={repeatIntervalS}
            onChange={(e) => setRepeatIntervalS(Number(e.target.value))}
            disabled={!repeatEnabled}
            className="mt-1 w-full bg-bg-card/80 border border-white/10 text-white rounded px-2 py-1.5 text-sm text-white disabled:opacity-40"
          />
        </label>
      </div>

      <div className="mb-3">
        <div className="flex items-center justify-between mb-1">
          <span className="text-xs text-white/70">
            选择目标（{selected.length}/{targets.length}）—— 建好后每台还能单独改阈值
          </span>
          <button
            onClick={() => setSelected(selected.length === targets.length ? [] : targets.map((t) => t.id))}
            className="text-xs text-accent hover:underline"
          >
            {selected.length === targets.length ? '全不选' : '全选当前全部'}
          </button>
        </div>
        <div className="max-h-48 overflow-auto grid md:grid-cols-3 gap-1">
          {targets.map((t) => (
            <label key={t.id} className="flex items-center gap-2 text-sm px-2 py-1 rounded hover:bg-bg-card/40">
              <input type="checkbox" checked={selected.includes(t.id)} onChange={() => toggle(t.id)} />
              <span className="truncate">{t.name}</span>
              <span className="text-white/40 text-xs truncate">{t.sub}</span>
            </label>
          ))}
          {targets.length === 0 && <span className="text-white/50 text-xs">还没有可选目标</span>}
        </div>
      </div>

      <button
        disabled={create.isPending || selected.length === 0 || !metric}
        onClick={() => create.mutate()}
        className="px-4 py-2 bg-accent text-black rounded-lg text-sm font-medium hover:bg-accent/80 disabled:opacity-40"
      >
        {create.isPending ? '创建中…' : '创建规则'}
      </button>
    </div>
  )
}

const OP_SYMBOL: Record<string, string> = { gt: '>', gte: '≥', lt: '<', lte: '≤' }

function RuleCard({
  rule,
  cls,
  metricDefs,
  allTargets,
  expanded,
  onToggle,
}: {
  rule: AlertRule
  cls: Class
  metricDefs: AlertMetricDef[]
  allTargets: { id: number; name: string; sub: string }[]
  expanded: boolean
  onToggle: () => void
}) {
  const qc = useQueryClient()
  const { data: detail } = useQuery({
    queryKey: ['alert-v2-detail', rule.id],
    queryFn: () => alertV2.detail(rule.id),
    // 卡片收起时也要知道覆盖几个目标/几台在告警，只是刷新频率低一些
    refetchInterval: expanded ? 15000 : 60000,
  })

  const removeRule = useMutation({
    mutationFn: async () => {
      const { alertRules } = await import('../api/client')
      return alertRules.delete(rule.id)
    },
    onSuccess: () => qc.invalidateQueries({ queryKey: ['alert-v2-rules', cls] }),
  })

  const def = metricDefs.find((m) => m.key === rule.metric_type)
  const op = OP_SYMBOL[rule.operator] || rule.operator
  const tracking = detail?.targets?.length ?? 0
  const firing = detail?.targets?.filter((t) => t.status === 'firing').length ?? 0

  return (
    <div className="bg-bg-card/70 rounded-xl px-4 py-3">
      <div className="flex items-center justify-between cursor-pointer" onClick={onToggle}>
        <div>
          <div className="flex items-center gap-2">
            <span className="font-medium">{rule.name}</span>
            <span className="text-[10px] px-1.5 py-0.5 rounded bg-white/10 text-white/70">
              {rule.kind === 'availability' ? '离线' : def?.label || rule.metric_type}
            </span>
            {firing > 0 && (
              <span className="text-[10px] px-1.5 py-0.5 rounded bg-err/20 text-err">
                {firing} 个目标告警中
              </span>
            )}
          </div>
          <div className="text-xs text-white/60 mt-1">
            {rule.metric_type === 'agent_offline'
              ? `离线 ${op} ${rule.threshold} 分钟`
              : `${def?.label || rule.metric_type} ${op} ${rule.threshold}${def?.unit === 'percent' ? '%' : def?.unit ? ' ' + def.unit : ''}`}
            {rule.duration_s > 0 ? ` · 持续 ${rule.duration_s}s` : ''}
            {` · 覆盖 ${tracking} 个目标`}
            {` · 防抖 ${rule.cooldown_s}s`}
          </div>
        </div>
        <div className="flex items-center gap-3">
          <button
            onClick={(e) => { e.stopPropagation(); if (confirm(`删除规则「${rule.name}」？`)) removeRule.mutate() }}
            className="text-err hover:underline text-xs"
          >
            删除
          </button>
          <span className="text-white/40 text-xs">{expanded ? '收起 ▲' : '展开 ▼'}</span>
        </div>
      </div>

      {expanded && (
        <div className="mt-3 border-t border-white/10 pt-3">
          {!detail && <div className="text-sm text-white/60 py-2">加载中…</div>}
          {detail && (
            <TargetTable rule={rule} detail={detail.targets} allTargets={allTargets} cls={cls} />
          )}
        </div>
      )}
    </div>
  )
}

function TargetTable({
  rule,
  detail,
  allTargets,
  cls,
}: {
  rule: AlertRule
  detail: AlertRuleTarget[]
  allTargets: { id: number; name: string; sub: string }[]
  cls: Class
}) {
  const qc = useQueryClient()
  const availability = rule.metric_type === 'agent_offline' || rule.kind === 'availability'

  const patch = useMutation({
    mutationFn: (v: { targetId: number; body: any }) => alertV2.patchTarget(rule.id, cls, v.targetId, v.body),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ['alert-v2-detail', rule.id] })
      qc.invalidateQueries({ queryKey: ['alert-v2-rules', cls] })
    },
  })
  const remove = useMutation({
    mutationFn: (targetId: number) => alertV2.removeTarget(rule.id, cls, targetId),
    onSuccess: () => qc.invalidateQueries({ queryKey: ['alert-v2-detail', rule.id] }),
  })
  const add = useMutation({
    mutationFn: (ids: number[]) => alertV2.addTargets(rule.id, { target_ids: ids }),
    onSuccess: () => qc.invalidateQueries({ queryKey: ['alert-v2-detail', rule.id] }),
  })

  const present = new Set(detail.map((t) => t.target_id))
  const missing = allTargets.filter((t) => !present.has(t.id))

  return (
    <div>
      <table className="w-full text-sm">
        <thead>
          <tr className="text-left text-white/60 text-xs border-b border-white/10">
            <th className="py-2">目标</th>
            <th className="py-2">当前值</th>
            <th className="py-2">{availability ? '阈值（分钟）' : '阈值'}</th>
            <th className="py-2">持续（秒）</th>
            <th className="py-2">状态</th>
            <th className="py-2 text-right">操作</th>
          </tr>
        </thead>
        <tbody>
          {detail.map((t) => (
            <tr key={`${t.target_type}-${t.target_id}`} className="border-b border-white/5">
              <td className="py-2">
                <div className="truncate">{t.target_name || `#${t.target_id}`}</div>
                <div className="text-[11px] text-white/40 truncate">{t.target_addr}</div>
              </td>
              <td className="py-2 text-white/80">
                {typeof t.current_value === 'number' ? t.current_value.toFixed(1) : '—'}
              </td>
              <td className="py-2">
                <input
                  type="number"
                  defaultValue={t.threshold ?? rule.threshold}
                  onBlur={(e) => {
                    const v = Number(e.target.value)
                    if (v !== (t.threshold ?? rule.threshold)) patch.mutate({ targetId: t.target_id, body: { threshold: v } })
                  }}
                  className="w-20 bg-bg-card/80 border border-white/10 text-white rounded px-2 py-1 text-sm"
                />
              </td>
              <td className="py-2">
                <input
                  type="number"
                  defaultValue={t.for_s ?? rule.duration_s}
                  onBlur={(e) => {
                    const v = Number(e.target.value)
                    if (v !== (t.for_s ?? rule.duration_s)) patch.mutate({ targetId: t.target_id, body: { for_s: v } })
                  }}
                  className="w-20 bg-bg-card/80 border border-white/10 text-white rounded px-2 py-1 text-sm"
                />
              </td>
              <td className={`py-2 ${STATUS_STYLE[t.status || 'ok']}`}>
                {STATUS_LABEL[t.status || 'ok']}
              </td>
              <td className="py-2 text-right">
                <button
                  onClick={() => patch.mutate({ targetId: t.target_id, body: { enabled: !t.enabled } })}
                  className="text-accent hover:underline text-xs mr-3"
                >
                  {t.enabled ? '停用' : '启用'}
                </button>
                <button
                  onClick={() => patch.mutate({
                    targetId: t.target_id,
                    body: { silenced_until: new Date(Date.now() + 2 * 3600 * 1000).toISOString() },
                  })}
                  className="text-warn hover:underline text-xs mr-3"
                >
                  静默2h
                </button>
                <button
                  onClick={() => { if (confirm('把该目标从这条规则移除？')) remove.mutate(t.target_id) }}
                  className="text-err hover:underline text-xs"
                >
                  移除
                </button>
              </td>
            </tr>
          ))}
        </tbody>
      </table>

      {missing.length > 0 && (
        <div className="mt-3 text-xs">
          <span className="text-white/60 mr-2">未纳入告警：</span>
          {missing.map((t) => (
            <button
              key={t.id}
              onClick={() => add.mutate([t.id])}
              className="mr-2 mb-1 px-2 py-0.5 rounded bg-white/10 hover:bg-accent/20 text-white/80"
              title={t.sub}
            >
              ＋ {t.name}
            </button>
          ))}
        </div>
      )}
    </div>
  )
}
