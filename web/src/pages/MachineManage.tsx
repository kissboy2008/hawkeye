import { useState } from 'react'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { agents, probes } from '../api/client'
import type { Agent, WebProbe } from '../types'
import {
 DndContext,
 closestCenter,
 KeyboardSensor,
 PointerSensor,
 useSensor,
 useSensors,
 type DragEndEvent,
} from '@dnd-kit/core'
import {
 SortableContext,
 sortableKeyboardCoordinates,
 verticalListSortingStrategy,
 useSortable,
} from '@dnd-kit/sortable'
import { CSS } from '@dnd-kit/utilities'
import { DragHandle } from '../components/probe/DragHandle'

const statusColors: Record<string, string> = {
 online: 'text-ok', offline: 'text-err', unknown: 'text-gray-400',
}

/* ========== Sortable Agent Row ========== */

function SortableAgentRow({ agent, onEdit, onDelete, onTest, testResult, testPending }: {
 agent: Agent
 onEdit: (agent: Agent) => void
 onDelete: (id: number) => void
 onTest: (id: number) => void
 testResult?: string
 testPending: boolean
}) {
 const {
  attributes,
  listeners,
  setNodeRef,
  transform,
  transition,
  isDragging,
 } = useSortable({ id: agent.id })

 const style: React.CSSProperties = {
  transform: CSS.Transform.toString(transform),
  transition,
  opacity: isDragging ? 0.5 : 1,
  zIndex: isDragging ? 10 : undefined,
  display: 'table-row' as const,
 }

 return (
  <tr ref={setNodeRef} style={style} {...attributes} className="grouphover:bg-bg-hover/30">
   <td className="px-4 py-3">
    <DragHandle listeners={listeners} />
   </td>
   <td className="px-4 py-3">
    <a href={`/machine/${encodeURIComponent(agent.name)}`} className="text-gray-200 hover:text-white hover:underline">{agent.name}</a>
   </td>
   <td className="px-4 py-3 text-gray-400 font-mono text-xs">{agent.address || <span className="text-gray-600">push 模式</span>}</td>
   <td className={`px-4 py-3 ${statusColors[agent.status] || ''}`}>
    {agent.status === 'online' ? '在线' : agent.status === 'offline' ? '离线' : '未知'}
   </td>
   <td className="px-4 py-3 pl-10 text-gray-500 text-xs font-mono">{agent.agent_version || '-'}</td>
   <td className="px-4 py-3 text-right">
    <div className="flex items-center justify-end gap-2">
     {agent.address && (
     <button
      onClick={() => onTest(agent.id)}
      disabled={testPending}
      className="px-3 py-1.5 rounded-lg text-xs bg-accent text-black font-medium hover:bg-accent/80 transition-colors disabled:opacity-50"
     >
      测试
     </button>
     )}
     <button
      onClick={() => onEdit(agent)}
      className="px-3 py-1.5 rounded-lg text-xs text-accent bg-accent/10 hover:bg-accent/20 transition-colors"
     >
      编辑
     </button>
     <button
      onClick={() => onDelete(agent.id)}
      className="px-3 py-1.5 rounded-lg text-xs text-err bg-err/10 hover:bg-err/20 transition-colors"
     >
      删除
     </button>
    </div>
    {testResult && (
     <div className={`text-xs mt-1 text-right ${testResult.startsWith('失败') ? 'text-err' : 'text-ok'}`}>
      {testResult}
     </div>
    )}
   </td>
  </tr>
 )
}

/* ========== Agent Section ========== */

function AgentSection() {
 const queryClient = useQueryClient()
 const { data: list = [] } = useQuery({ queryKey: ['agents'], queryFn: agents.list })
 const [editing, setEditing] = useState<Partial<Agent> | null>(null)
 const [testResult, setTestResult] = useState<Record<number, string>>({})
 const [saveMsg, setSaveMsg] = useState<string | null>(null)

 const create = useMutation({
  mutationFn: (data: Partial<Agent>) => agents.create(data),
  onSuccess: () => { queryClient.invalidateQueries({ queryKey: ['agents'] }) },
 })
 const update = useMutation({
  mutationFn: ({ id, ...data }: Partial<Agent> & { id: number }) => agents.update(id, data),
  onSuccess: () => { queryClient.invalidateQueries({ queryKey: ['agents'] }) },
 })
 const pushServerURL = useMutation({
  mutationFn: ({ id, server_url }: { id: number; server_url: string }) =>
   agents.pushServerURL(id, server_url),
  onSuccess: (res) => {
   setSaveMsg(res.message)
   setTimeout(() => setSaveMsg(null), 6000)
  },
  onError: (err: Error) => {
   setSaveMsg('推送失败: ' + err.message)
   setTimeout(() => setSaveMsg(null), 6000)
  },
 })
 const pushAuthToken = useMutation({
  mutationFn: ({ id, auth_token, old_token }: { id: number; auth_token: string; old_token?: string }) =>
   agents.pushAuthToken(id, auth_token, old_token),
  onSuccess: (res) => {
   setSaveMsg(res.message)
   setTimeout(() => setSaveMsg(null), 6000)
  },
  onError: (err: Error) => {
   setSaveMsg('推送 Token 失败: ' + err.message)
   setTimeout(() => setSaveMsg(null), 6000)
  },
 })
 const remove = useMutation({
  mutationFn: (id: number) => agents.delete(id),
  onSuccess: () => queryClient.invalidateQueries({ queryKey: ['agents'] }),
 })
 const test = useMutation({
  mutationFn: (id: number) => agents.test(id),
  onSuccess: (res, id) => {
   setTestResult((prev) => ({ ...prev, [id]: res.success ? '连接成功' : `失败: ${res.error}` }))
  },
 })
 const agentReorder = useMutation({
  mutationFn: (ids: number[]) => agents.reorder(ids),
  onSuccess: () => queryClient.invalidateQueries({ queryKey: ['agents'] }),
 })

 const handleSubmit = (e: React.FormEvent) => {
  e.preventDefault()
  if (!editing) return
  const mode = editing.mode || 'push'
  const originalAgent = editing.id ? list.find((a) => a.id === editing.id) : null
  const tokenChanged = !!(originalAgent && editing.auth_token && editing.auth_token !== (originalAgent.auth_token || ''))

  // Build data - always include auth_token
      const data: Record<string, unknown> = {
       name: editing.name!,
       address: editing.address || '',
       server_url: editing.server_url || '',
       auth_token: editing.auth_token || '',
       intranet_url: editing.intranet_url || '',
       extranet_url: editing.extranet_url || '',
       mode,
      }
  // Mode-based validation
  if (mode === 'pull' && !(data.address as string).trim()) {
   setSaveMsg('Pull 模式下 Agent 地址必须填写')
   return
  }
  if (mode === 'push' && !(data.server_url as string).trim()) {
   setSaveMsg('Push 模式下 Server 地址必须填写')
   return
  }

  setSaveMsg('保存中...')

  const testConnection = (agentId: number) => {
   agents.test(agentId).then((res) => {
    queryClient.invalidateQueries({ queryKey: ['agents'] })
    if (res.success) {
     setSaveMsg('保存成功，连接正常')
     setTimeout(() => setEditing(null), 600)
    } else {
     setSaveMsg('保存成功但连接失败: ' + (res.error || '无法连接 Agent'))
    }
   }).catch((err: Error) => {
    setSaveMsg('连接测试失败: ' + err.message)
   })
  }

  const afterSave = (agentId: number) => {
   // Push auth token first if changed
   if (tokenChanged) {
    pushAuthToken.mutate(
     { id: agentId, auth_token: editing.auth_token!, old_token: originalAgent?.auth_token || '' },
     {
      onSuccess: () => {
       // After token pushed, push server URL for push mode
       if (mode === 'push' && data.server_url) {
        pushServerURL.mutate({ id: agentId, server_url: data.server_url as string }, {
         onSuccess: () => {
          if ((data.address as string).trim()) {
           testConnection(agentId)
          } else {
           queryClient.invalidateQueries({ queryKey: ['agents'] })
           setSaveMsg('保存成功（无 Agent 地址，跳过连接测试）')
           setTimeout(() => setEditing(null), 800)
          }
         },
         onError: (err) => {
          setSaveMsg('推送 Server URL 失败: ' + err.message)
         },
        })
       } else if ((data.address as string).trim()) {
        testConnection(agentId)
       } else {
        queryClient.invalidateQueries({ queryKey: ['agents'] })
        setSaveMsg('保存成功')
        setTimeout(() => setEditing(null), 600)
       }
      },
      onError: (err) => {
       setSaveMsg('推送 Token 失败: ' + err.message)
      },
     }
    )
   } else if (mode === 'push' && data.server_url) {
    pushServerURL.mutate({ id: agentId, server_url: data.server_url as string }, {
     onSuccess: () => {
      if ((data.address as string).trim()) {
       testConnection(agentId)
      } else {
       queryClient.invalidateQueries({ queryKey: ['agents'] })
       setSaveMsg('保存成功（无 Agent 地址，跳过连接测试）')
       setTimeout(() => setEditing(null), 800)
      }
     },
     onError: (err) => {
      setSaveMsg('推送 Server URL 失败: ' + err.message)
     },
    })
   } else if ((data.address as string).trim()) {
    testConnection(agentId)
   } else {
    queryClient.invalidateQueries({ queryKey: ['agents'] })
    setSaveMsg('保存成功')
    setTimeout(() => setEditing(null), 600)
   }
  }

  if (editing.id) {
   update.mutate({ id: editing.id, ...data } as Partial<Agent> & { id: number }, {
    onSuccess: () => afterSave(editing.id!),
    onError: (err) => setSaveMsg('保存失败: ' + err.message),
   })
  } else {
   create.mutate(data as Partial<Agent>, {
    onSuccess: (newAgent) => {
     if (newAgent?.id) afterSave(newAgent.id)
     else {
      queryClient.invalidateQueries({ queryKey: ['agents'] })
      setSaveMsg('保存成功')
      setTimeout(() => setEditing(null), 600)
     }
    },
    onError: (err) => setSaveMsg('保存失败: ' + err.message),
   })
  }
 }

 const sensors = useSensors(
  useSensor(PointerSensor, { activationConstraint: { distance: 5 } }),
  useSensor(KeyboardSensor, { coordinateGetter: sortableKeyboardCoordinates }),
 )

 function handleDragEnd(event: DragEndEvent) {
  const { active, over } = event
  if (!over || active.id === over.id) return
  const oldIndex = list.findIndex((a) => a.id === active.id)
  const newIndex = list.findIndex((a) => a.id === over.id)
  if (oldIndex === -1 || newIndex === -1) return
  const newList = [...list]
  const [moved] = newList.splice(oldIndex, 1)
  newList.splice(newIndex, 0, moved)
  agentReorder.mutate(newList.map((a) => a.id))
 }

 return (
  <>
   <div className="flex items-center justify-between mb-4">
    <div className="flex items-center gap-2">
     <span className="text-base">⊞</span>
     <h3 className="text-lg font-semibold">主机监控</h3>
     <span className="text-xs text-gray-500">{list.length} 台</span>
    </div>
    <button
     onClick={() => setEditing({ name: '', address: '', auth_token: '', server_url: '' })}
     className="px-3 py-1.5 bg-accent text-black rounded-lg text-xs font-medium hover:bg-accent/80 transition-colors"
    >
     + 添加主机
    </button>
   </div>

   <div className="bg-bg-card/70 rounded-xl overflow-x-auto mb-8">
    <DndContext sensors={sensors} collisionDetection={closestCenter} onDragEnd={handleDragEnd}>
     <SortableContext items={list.map((a) => a.id)} strategy={verticalListSortingStrategy}>
      <table className="w-full text-sm min-w-[500px]">
       <thead>
        <tr className="border-b text-left text-gray-500">
         <th className="px-4 py-3 w-8"></th>
         <th className="px-4 py-3">名称</th>
         <th className="px-4 py-3">地址</th>
         <th className="px-4 py-3">状态</th>
         <th className="px-4 py-3 pl-10">版本</th>
         <th className="px-4 py-3 text-right">操作</th>
        </tr>
       </thead>
       <tbody>
        {list.map((agent) => (
         <SortableAgentRow
          key={agent.id}
          agent={agent}
          onEdit={(a) => setEditing(a)}
          onDelete={(id) => { if (confirm('确认删除?')) remove.mutate(id) }}
          onTest={(id) => test.mutate(id)}
          testResult={testResult[agent.id]}
          testPending={test.isPending}
         />
        ))}
       </tbody>
      </table>
     </SortableContext>
    </DndContext>
    {list.length === 0 && (
     <div className="py-10 text-center text-gray-500 text-sm">暂无主机</div>
    )}
   </div>

   {editing && (
    <div className="fixed inset-0 bg-black/50 flex items-center justify-center z-50">
     <div className="bg-bg-card/70 rounded-xl p-4 md:p-6 w-[calc(100%-2rem)] max-w-md mx-4">
      <h3 className="text-lg font-semibold mb-4">{editing.id ? '编辑主机' : '添加主机'}</h3>
      <form onSubmit={handleSubmit} className="space-y-3">
       <div>
        <label className="text-xs text-gray-400 block mb-1">名称</label>
        <input
         className="w-full bg-bg px-3 py-2 rounded-lg border text-sm focus:border-accent outline-none"
         value={editing.name || ''} onChange={(e) => setEditing({ ...editing, name: e.target.value })} required
        />
       </div>
       <div>
        <label className="text-xs text-gray-400 block mb-1">
         Agent 地址
         {(editing.mode || 'push') === 'push'
          ? <span className="text-gray-600"> (push 模式下无需填写)</span>
          : <span className="text-err"> *</span>
         }
        </label>
        <input
         className={`w-full bg-bg px-3 py-2 rounded-lg border text-sm outline-none font-mono ${(editing.mode || 'push') === 'push' ? 'text-gray-500 opacity-50 cursor-not-allowed' : 'focus:border-accent'}`}
         value={editing.address || ''}
         onChange={(e) => setEditing({ ...editing, address: e.target.value })}
         placeholder="http://0.0.0.0:32518"
         disabled={(editing.mode || 'push') === 'push'}
         required={(editing.mode || 'push') === 'pull'}
        />
       </div>
       <div>
        <label className="text-xs text-gray-400 block mb-1">
         Server 地址
         {(editing.mode || 'push') === 'push'
          ? <span className="text-err"> *</span>
          : <span className="text-gray-600"> (pull 模式下无需填写)</span>
         }
        </label>
        <input
         className={`w-full bg-bg px-3 py-2 rounded-lg border text-sm outline-none font-mono ${(editing.mode || 'push') === 'pull' ? 'text-gray-500 opacity-50 cursor-not-allowed' : 'focus:border-accent'}`}
         value={editing.server_url || ''}
         onChange={(e) => setEditing({ ...editing, server_url: e.target.value })}
         placeholder="http://0.0.0.0:18325"
         disabled={(editing.mode || 'push') === 'pull'}
         required={(editing.mode || 'push') === 'push'}
        />
       </div>
       <div>
        <label className="text-xs text-gray-400 block mb-1">Auth Token <span className="text-err">*</span></label>
        <input
         className="w-full bg-bg px-3 py-2 rounded-lg border text-sm focus:border-accent outline-none"
         value={editing.auth_token || ''} onChange={(e) => setEditing({ ...editing, auth_token: e.target.value })}
         required
        />
       </div>
       <div>
        <label className="text-xs text-gray-400 block mb-1">工作模式</label>
        <div className="flex gap-3">
         <label className="flex items-center gap-1.5 cursor-pointer">
          <input
           type="radio" name="mode" value="push"
           checked={(editing.mode || 'push') === 'push'}
           onChange={() => setEditing({ ...editing, mode: 'push' })}
           className="accent-[var(--color-accent)]"
          />
          <span className="text-sm">Push（Agent 主动推送）</span>
         </label>
         <label className="flex items-center gap-1.5 cursor-pointer">
          <input
           type="radio" name="mode" value="pull"
           checked={editing.mode === 'pull'}
           onChange={() => setEditing({ ...editing, mode: 'pull' })}
           className="accent-[var(--color-accent)]"
          />
          <span className="text-sm">Pull（服务端主动拉取）</span>
         </label>
        </div>
       </div>
       <div>
        <label className="text-xs text-gray-400 block mb-1">内网地址 <span className="text-gray-600">(左键跳转)</span></label>
        <input
         className="w-full bg-bg px-3 py-2 rounded-lg border text-sm focus:border-accent outline-none font-mono"
         value={editing.intranet_url || ''} onChange={(e) => setEditing({ ...editing, intranet_url: e.target.value })}
         placeholder="http://192.168.1.1"
        />
       </div>
       <div>
        <label className="text-xs text-gray-400 block mb-1">外网地址 <span className="text-gray-600">(右键跳转)</span></label>
        <input
         className="w-full bg-bg px-3 py-2 rounded-lg border text-sm focus:border-accent outline-none font-mono"
         value={editing.extranet_url || ''} onChange={(e) => setEditing({ ...editing, extranet_url: e.target.value })}
         placeholder="https://xxx.example.com"
        />
       </div>
       <div className="flex gap-3 pt-2">
        <button type="submit" className="flex-1 bg-accent text-black rounded-lg py-2 text-sm font-medium hover:bg-accent/80">
         {editing.id ? '保存' : '添加'}
        </button>
        <button type="button" onClick={() => setEditing(null)} className="flex-1 bg-bg-hover rounded-lg py-2 text-sm">取消</button>
       </div>
       {saveMsg && (
        <div className={`mt-3 p-2.5 rounded-lg text-xs ${
         saveMsg.includes('失败') || saveMsg.includes('推送失败')
          ? 'bg-err/10 text-err border border-err/20'
          : 'bg-ok/10 text-ok border border-ok/20'
        }`}>
         {saveMsg}
        </div>
       )}
      </form>
     </div>
    </div>
   )}
  </>
 )
}

/* ========== Sortable Probe Row ========== */

function SortableProbeRow({ probe, onEdit, onDelete, onTest, testResult, testPending }: {
 probe: WebProbe
 onEdit: (probe: WebProbe) => void
 onDelete: (id: number) => void
 onTest: (id: number) => void
 testResult?: string
 testPending: boolean
}) {
 const {
  attributes,
  listeners,
  setNodeRef,
  transform,
  transition,
  isDragging,
 } = useSortable({ id: probe.id })

 const style: React.CSSProperties = {
  transform: CSS.Transform.toString(transform),
  transition,
  opacity: isDragging ? 0.5 : 1,
  zIndex: isDragging ? 10 : undefined,
  display: 'table-row' as const,
 }

 const { data: results } = useQuery({
  queryKey: ['probe-results', probe.id],
  queryFn: () => probes.results(probe.id, 1),
  refetchInterval: 60000,
 })
 const last = results && results.length > 0 ? results[0] : null

 return (
  <tr ref={setNodeRef} style={style} {...attributes} className="grouphover:bg-bg-hover/30">
   <td className="px-4 py-3">
    <DragHandle listeners={listeners} />
   </td>
   <td className="px-4 py-3 font-medium">{probe.name}</td>
   <td className="px-4 py-3 text-gray-400 font-mono text-xs truncate max-w-[300px]">{probe.url}</td>
   <td className="px-4 py-3">
    {!last ? <span className="text-gray-500 text-xs">未检测</span>
     : last.success ? <span className="text-ok text-xs">在线（{last.status_code}）</span>
     : <span className="text-err text-xs">异常（{last.status_code || '-'}）</span>}
   </td>
   <td className="px-4 py-3 text-gray-400 text-xs">{probe.interval_s}s</td>
   <td className="px-4 py-3 text-right">
    <div className="flex items-center justify-end gap-2">
     <button
      onClick={() => onTest(probe.id)}
      disabled={testPending}
      className="px-3 py-1.5 rounded-lg text-xs bg-accent text-black font-medium hover:bg-accent/80 transition-colors disabled:opacity-50"
     >
      测试
     </button>
     <button
      onClick={() => onEdit(probe)}
      className="px-3 py-1.5 rounded-lg text-xs text-accent bg-accent/10 hover:bg-accent/20 transition-colors"
     >
      编辑
     </button>
     <button
      onClick={() => onDelete(probe.id)}
      className="px-3 py-1.5 rounded-lg text-xs text-err bg-err/10 hover:bg-err/20 transition-colors"
     >
      删除
     </button>
    </div>
    {testResult && (
     <div className={`text-xs mt-1 text-right ${testResult.startsWith('失败') ? 'text-err' : 'text-ok'}`}>
      {testResult}
     </div>
    )}
   </td>
  </tr>
 )
}

/* ========== Probe Section ========== */

function ProbeSection() {
 const queryClient = useQueryClient()
 const { data: list = [] } = useQuery({ queryKey: ['probes'], queryFn: probes.list })
 const [editing, setEditing] = useState<Partial<WebProbe> | null>(null)
 const [testResult, setTestResult] = useState<Record<number, string>>({})

 const create = useMutation({
  mutationFn: (data: Partial<WebProbe>) => probes.create(data),
  onSuccess: () => { queryClient.invalidateQueries({ queryKey: ['probes'] }); setEditing(null) },
 })
 const update = useMutation({
  mutationFn: ({ id, ...data }: Partial<WebProbe> & { id: number }) => probes.update(id, data),
  onSuccess: () => { queryClient.invalidateQueries({ queryKey: ['probes'] }); setEditing(null) },
 })
 const remove = useMutation({
  mutationFn: (id: number) => probes.delete(id),
  onSuccess: () => queryClient.invalidateQueries({ queryKey: ['probes'] }),
 })
 const test = useMutation({
  mutationFn: (id: number) => probes.check(id),
  onSuccess: (res, id) => {
   setTestResult((prev) => ({
    ...prev,
    [id]: res.success
     ? `成功 ${res.status_code} - ${res.latency_ms}ms`
     : `失败: ${res.error}`,
   }))
  },
  onError: (err, id) => {
   setTestResult((prev) => ({ ...prev, [id]: `失败: ${err.message}` }))
  },
 })
 const probeReorder = useMutation({
  mutationFn: (ids: number[]) => probes.reorder(ids),
  onSuccess: () => queryClient.invalidateQueries({ queryKey: ['probes'] }),
 })

 const handleSubmit = (e: React.FormEvent) => {
  e.preventDefault()
  if (!editing) return
  const data = {
   name: editing.name!,
   url: editing.url!,
   method: editing.method || 'GET',
   expected_status: editing.expected_status || 200,
   timeout_ms: editing.timeout_ms || 5000,
   interval_s: editing.interval_s || 60,
  }
  if (editing.id) {
   update.mutate({ id: editing.id, ...data })
  } else {
   create.mutate(data)
  }
 }

 const sensors = useSensors(
  useSensor(PointerSensor, { activationConstraint: { distance: 5 } }),
  useSensor(KeyboardSensor, { coordinateGetter: sortableKeyboardCoordinates }),
 )

 function handleDragEnd(event: DragEndEvent) {
  const { active, over } = event
  if (!over || active.id === over.id) return
  const oldIndex = list.findIndex((p) => p.id === active.id)
  const newIndex = list.findIndex((p) => p.id === over.id)
  if (oldIndex === -1 || newIndex === -1) return
  const newList = [...list]
  const [moved] = newList.splice(oldIndex, 1)
  newList.splice(newIndex, 0, moved)
  probeReorder.mutate(newList.map((p) => p.id))
 }

 return (
  <>
   <div className="flex items-center justify-between mb-4">
    <div className="flex items-center gap-2">
     <span className="text-base">◈</span>
     <h3 className="text-lg font-semibold">网站监控</h3>
     <span className="text-xs text-gray-500">{list.length} 个</span>
    </div>
    <button
     onClick={() => setEditing({ name: '', url: 'https://', method: 'GET', expected_status: 200, timeout_ms: 5000, interval_s: 60 })}
     className="px-3 py-1.5 bg-accent text-black rounded-lg text-xs font-medium hover:bg-accent/80 transition-colors"
    >
     + 添加网站
    </button>
   </div>

   <div className="bg-bg-card/70 rounded-xl overflow-x-auto">
    <DndContext sensors={sensors} collisionDetection={closestCenter} onDragEnd={handleDragEnd}>
     <SortableContext items={list.map((p) => p.id)} strategy={verticalListSortingStrategy}>
      <table className="w-full text-sm min-w-[500px]">
       <thead>
        <tr className="border-b text-left text-gray-500">
         <th className="px-4 py-3 w-8"></th>
         <th className="px-4 py-3">名称</th>
         <th className="px-4 py-3">URL</th>
         <th className="px-4 py-3">状态</th>
         <th className="px-4 py-3">间隔</th>
         <th className="px-4 py-3 text-right">操作</th>
        </tr>
       </thead>
       <tbody>
        {list.map((probe) => (
         <SortableProbeRow
          key={probe.id}
          probe={probe}
          onEdit={(p) => setEditing(p)}
          onDelete={(id) => { if (confirm('确认删除?')) remove.mutate(id) }}
          onTest={(id) => test.mutate(id)}
          testResult={testResult[probe.id]}
          testPending={test.isPending}
         />
        ))}
       </tbody>
      </table>
     </SortableContext>
    </DndContext>
    {list.length === 0 && (
     <div className="py-10 text-center text-gray-500 text-sm">暂无网站</div>
    )}
   </div>

   {editing && (
    <div className="fixed inset-0 bg-black/50 flex items-center justify-center z-50">
     <div className="bg-bg-card/70 rounded-xl p-4 md:p-6 w-[calc(100%-2rem)] max-w-md mx-4">
      <h3 className="text-lg font-semibold mb-4">{editing.id ? '编辑网站' : '添加网站'}</h3>
      <form onSubmit={handleSubmit} className="space-y-3">
       <div>
        <label className="text-xs text-gray-400 block mb-1">名称</label>
        <input
         className="w-full bg-bg px-3 py-2 rounded-lg border text-sm focus:border-accent outline-none"
         value={editing.name || ''} onChange={(e) => setEditing({ ...editing, name: e.target.value })} required
        />
       </div>
       <div>
        <label className="text-xs text-gray-400 block mb-1">URL</label>
        <input
         className="w-full bg-bg px-3 py-2 rounded-lg border text-sm focus:border-accent outline-none font-mono"
         value={editing.url || ''} onChange={(e) => setEditing({ ...editing, url: e.target.value })} required
        />
       </div>
       <div className="grid grid-cols-2 gap-3">
        <div>
         <label className="text-xs text-gray-400 block mb-1">请求方式</label>
         <select
          className="w-full bg-bg px-3 py-2 rounded-lg border text-sm focus:border-accent outline-none"
          value={editing.method || 'GET'} onChange={(e) => setEditing({ ...editing, method: e.target.value })}
         >
          <option>GET</option><option>HEAD</option>
         </select>
        </div>
        <div>
         <label className="text-xs text-gray-400 block mb-1">预期状态码</label>
         <input
          className="w-full bg-bg px-3 py-2 rounded-lg border text-sm focus:border-accent outline-none"
          type="number" value={editing.expected_status || 200} onChange={(e) => setEditing({ ...editing, expected_status: Number(e.target.value) })}
         />
        </div>
       </div>
       <div className="grid grid-cols-2 gap-3">
        <div>
         <label className="text-xs text-gray-400 block mb-1">检测间隔（秒）</label>
         <input
          className="w-full bg-bg px-3 py-2 rounded-lg border text-sm focus:border-accent outline-none"
          type="number" value={editing.interval_s || 60} onChange={(e) => setEditing({ ...editing, interval_s: Number(e.target.value) })}
         />
        </div>
        <div>
         <label className="text-xs text-gray-400 block mb-1">超时（毫秒）</label>
         <input
          className="w-full bg-bg px-3 py-2 rounded-lg border text-sm focus:border-accent outline-none"
          type="number" value={editing.timeout_ms || 5000} onChange={(e) => setEditing({ ...editing, timeout_ms: Number(e.target.value) })}
         />
        </div>
       </div>
       <div className="flex gap-3 pt-2">
        <button type="submit" className="flex-1 bg-accent text-black rounded-lg py-2 text-sm font-medium hover:bg-accent/80">
         {editing.id ? '保存' : '添加'}
        </button>
        <button type="button" onClick={() => setEditing(null)} className="flex-1 bg-bg-hover rounded-lg py-2 text-sm">取消</button>
       </div>
      </form>
     </div>
    </div>
   )}
  </>
 )
}

/* ========== Main Page ========== */

export default function MonitorManage() {
 return (
  <div className="py-4 px-2 md:py-6 md:px-3 max-w-[1920px] mx-auto">
   <div className="mb-4 md:mb-6">
    <h2 className="text-xl md:text-2xl font-bold mb-1">监控管理</h2>
    <p className="text-gray-500 text-sm">管理主机和网站监控配置</p>
   </div>

   <AgentSection />
   <ProbeSection />
  </div>
 )
}
