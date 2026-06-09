import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { widgets, type Widget } from '../api/client'
import { useState, useMemo, useEffect } from 'react'
import {
 DndContext,
 closestCorners,
 PointerSensor,
 useSensor,
 useSensors,
 useDroppable,
 type DragEndEvent,
} from '@dnd-kit/core'
import {
 SortableContext,
 useSortable,
 verticalListSortingStrategy,
} from '@dnd-kit/sortable'
import { CSS } from '@dnd-kit/utilities'
import { renderWidget, WidgetSkeleton, WidgetError } from '../components/WidgetCards'


// --- Add Widget Dialog ---

// --- Add Widget Dialog ---
function AddWidgetDialog({ open, onClose, onSave, existingGroups }: { open: boolean; onClose: () => void; onSave: (w: Partial<Widget>) => void; existingGroups: string[] }) {
 const [form, setForm] = useState<Partial<Widget>>({ type: '', name: '', url: '', api_token: '', node: '', widget_group: '' })
 const [newGroup, setNewGroup] = useState('')
 const [customEntities, setCustomEntities] = useState<{ entity_id: string; label: string }[]>([{ entity_id: '', label: '' }])
 const [sshHost, setSshHost] = useState('10.0.0.1')
 const [sshPort, setSshPort] = useState('22')
 const [sshUser, setSshUser] = useState('root')
 const [sshPassword, setSshPassword] = useState('')

 const typeOptions = [
  { value: 'adguard', label: 'AdGuard Home', placeholder: 'http://192.168.1.53' },
  { value: 'hawkeye', label: 'Hawkeye', placeholder: '无需填写' },
  { value: 'homeassistant', label: 'Home Assistant', placeholder: 'http://192.168.1.x:8123' },
  { value: 'jellyfin', label: 'Jellyfin', placeholder: 'http://192.168.1.100:8096' },
  { value: 'lucky', label: 'Lucky', placeholder: 'http://192.168.1.100:16666' },
  { value: 'moviepilot', label: 'MoviePilot', placeholder: 'http://192.168.1.100:3000/api/v1/plugin/HomePage/statistic' },
  { value: 'openwrt', label: 'OpenWrt', placeholder: 'http://192.168.1.1' },
   { value: 'ikuai', label: 'iKuai', placeholder: 'http://10.10.10.1' },
   { value: 'openclash', label: 'OpenClash', placeholder: 'http://10.0.0.1:9090' },
   { value: 'pbs', label: 'Proxmox Backup Server', placeholder: 'https://192.168.1.99:8007' },  { value: 'portainer', label: 'Portainer', placeholder: 'https://192.168.1.100:9443' },
  { value: 'proxmox', label: 'Proxmox VE', placeholder: 'https://192.168.1.200:8006' },
  { value: 'qbittorrent', label: 'qBittorrent', placeholder: 'http://192.168.1.100:8080' },
  { value: 'transmission', label: 'Transmission', placeholder: 'http://192.168.1.100:9091' },
  { value: 'unraid', label: 'Unraid', placeholder: 'http://192.168.1.100' },
 ]

 const currentType = typeOptions.find(t => t.value === form.type)

 // 选择类型后自动填入名称
 useEffect(() => {
  if (form.type) {
   const selected = typeOptions.find(t => t.value === form.type)
   if (selected) setForm(prev => ({ ...prev, name: selected.label }))
  }
 }, [form.type])

 if (!open) return null

 return (
  <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/60 backdrop-blur-sm">
   <div className="bg-bg-card/70 rounded-xl p-6 w-full max-w-md shadow-glow">
    <h3 className="text-lg font-bold mb-4">添加小组件</h3>

    <div className="space-y-3">
     <div>
      <label className="text-xs text-white block mb-1">类型</label>
      <select
       className="w-full bg-bg-hover border rounded-lg px-3 py-2 text-sm"
       value={form.type}
       onChange={e => setForm({ ...form, type: e.target.value })}
      >
       <option value="">请选择...</option>
       {typeOptions.map(t => <option key={t.value} value={t.value}>{t.label}</option>)}
      </select>
     </div>

     <div>
      <label className="text-xs text-white block mb-1">名称</label>
      <input
       className="w-full bg-bg-hover border rounded-lg px-3 py-2 text-sm"
       placeholder="例如: PVE-MS01"
       value={form.name || ''}
       onChange={e => setForm({ ...form, name: e.target.value })}
      />
     </div>

     <div>
      <label className="text-xs text-white block mb-1">地址</label>
      <input
       className="w-full bg-bg-hover border rounded-lg px-3 py-2 text-sm"
       placeholder={currentType?.placeholder}
       value={form.url || ''}
       onChange={e => setForm({ ...form, url: e.target.value })}
      />
     </div>

     <div>
      <label className="text-xs text-white block mb-1">API Token</label>
      <input
       className="w-full bg-bg-hover border rounded-lg px-3 py-2 text-sm"
       placeholder={
        ['adguard', 'qbittorrent', 'transmission'].includes(form.type || '') ? '用户名:密码' :
        form.type === 'proxmox' ? 'user@pam!tokenname=xxx' :
        form.type === 'pbs' ? 'user@pbs!tokenname=xxx' :
        form.type === 'unraid' ? 'Unraid API Key' :
        form.type === 'portainer' ? 'ptr_xxx' :
        form.type === 'jellyfin' ? 'Jellyfin API Key' :
        form.type === 'moviepilot' ? 'MoviePilot API Key' :
        form.type === 'lucky' ? 'OpenToken' :
        form.type === 'homeassistant' ? 'HA 长期访问令牌' :
        '请输入 API Token'
       }
       value={form.api_token || ''}
       onChange={e => setForm({ ...form, api_token: e.target.value })}
      />
      <p className="text-xs text-white mt-1">
       {form.type === 'proxmox' && 'PVE格式: user@pam!tokenname=xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx'}
       {form.type === 'pbs' && 'PBS格式: user@pbs!tokenname=xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx'}
       {form.type === 'unraid' && 'Unraid Connect API Key'}
       {form.type === 'portainer' && 'Portainer API Key (ptr_xxx)'}
       {form.type === 'adguard' && '格式: 用户名:密码'}
       {form.type === 'jellyfin' && 'Jellyfin API Key'}
       {form.type === 'moviepilot' && 'MoviePilot API Key'}
       {form.type === 'qbittorrent' && '格式: 用户名:密码'}
       {form.type === 'lucky' && 'Lucky OpenToken (设置→高级选项)'}
       {form.type === 'homeassistant' && 'HA 用户长期访问令牌 (个人资料→安全→长期访问令牌)'}
       {form.type === 'transmission' && '格式: 用户名:密码'}
      </p>
     </div>

     {(form.type === 'proxmox' || form.type === 'pbs') && (
     <div>
      <label className="text-xs text-white block mb-1">
       {form.type === 'pbs' ? '数据存储名称' : '节点名称'}
      </label>
      <input
       className="w-full bg-bg-hover border rounded-lg px-3 py-2 text-sm"
       placeholder={form.type === 'pbs' ? 'PBS' : 'PVE-MS-01'}
       value={form.node || ''}
       onChange={e => setForm({ ...form, node: e.target.value })}
      />
     </div>
     )}

     {form.type === 'openclash' && (
     <div className="space-y-3 border-t border-white/10 pt-3 mt-3">
      <p className="text-xs text-white/70 font-medium">SSH 控制（用于启停 OpenClash 服务）</p>
      <div>
       <label className="text-xs text-white block mb-1">路由器 IP</label>
       <input className="w-full bg-bg-hover border rounded-lg px-3 py-2 text-sm" placeholder="10.0.0.1" value={sshHost} onChange={e => setSshHost(e.target.value)} />
      </div>
      <div className="flex gap-2">
       <div className="flex-1">
        <label className="text-xs text-white block mb-1">SSH 端口</label>
        <input className="w-full bg-bg-hover border rounded-lg px-3 py-2 text-sm" placeholder="22" value={sshPort} onChange={e => setSshPort(e.target.value)} />
       </div>
       <div className="flex-1">
        <label className="text-xs text-white block mb-1">SSH 用户名</label>
        <input className="w-full bg-bg-hover border rounded-lg px-3 py-2 text-sm" placeholder="root" value={sshUser} onChange={e => setSshUser(e.target.value)} />
       </div>
      </div>
      <div>
       <label className="text-xs text-white block mb-1">SSH 密码</label>
       <input type="password" className="w-full bg-bg-hover border rounded-lg px-3 py-2 text-sm" placeholder="路由器 SSH 密码" value={sshPassword} onChange={e => setSshPassword(e.target.value)} />
      </div>
     </div>
     )}

     {form.type === 'homeassistant' && (
     <div>
      <label className="text-xs text-white block mb-1">自定义监控实体（可选）</label>
      <p className="text-xs text-white mb-2">输入 Home Assistant 的 entity_id，最多 4 个</p>
      {customEntities.map((ce, i) => (
       <div key={i} className="flex gap-2 mb-1.5">
        <input
         className="flex-1 bg-bg-hover border rounded-lg px-3 py-1.5 text-xs"
         placeholder="例: sensor.total_power"
         value={ce.entity_id}
         onChange={e => {
          const updated = [...customEntities]
          updated[i] = { ...updated[i], entity_id: e.target.value }
          setCustomEntities(updated)
         }}
        />
        <input
         className="w-28 bg-bg-hover border rounded-lg px-3 py-1.5 text-xs"
         placeholder="标签"
         value={ce.label}
         onChange={e => {
          const updated = [...customEntities]
          updated[i] = { ...updated[i], label: e.target.value }
          setCustomEntities(updated)
         }}
        />
        {customEntities.length > 1 && (
         <button
          onClick={() => setCustomEntities(prev => prev.filter((_, idx) => idx !== i))}
          className="text-red-400 hover:text-red-300 px-1 text-xs"
         >
          ✕
         </button>
        )}
       </div>
      ))}
      {customEntities.length < 4 && customEntities[customEntities.length - 1]?.entity_id && (
       <button
        onClick={() => setCustomEntities(prev => [...prev, { entity_id: '', label: '' }])}
        className="text-xs text-blue-400 hover:text-blue-300 mt-1"
       >
        + 添加实体
       </button>
      )}
     </div>
     )}

     <div>
      <label className="text-xs text-white block mb-1">分组</label>
      <div className="flex gap-2">
       <select
        className="flex-1 bg-bg-hover border rounded-lg px-3 py-2 text-sm"
        value={form.widget_group || ''}
        onChange={e => { setForm({ ...form, widget_group: e.target.value }); setNewGroup('') }}
       >
        <option value="">未分组</option>
        {existingGroups.map(g => <option key={g} value={g}>{g}</option>)}
       </select>
       <input
        className="flex-1 bg-bg-hover border rounded-lg px-3 py-2 text-sm"
        placeholder="或输入新分组名"
        value={newGroup}
        onChange={e => setNewGroup(e.target.value)}
       />
      </div>
     </div>

     <div>
      <label className="text-xs text-white block mb-1">描述（可选）</label>
      <input
       className="w-full bg-bg-hover border rounded-lg px-3 py-2 text-sm"
       placeholder="显示在名称下方的备注信息"
       value={form.description || ''}
       onChange={e => setForm({ ...form, description: e.target.value })}
      />
     </div>
    </div>

    <div className="flex justify-end gap-2 mt-6">
     <button
      onClick={onClose}
      className="px-4 py-2 text-sm text-white hover:text-white transition-colors"
     >
      取消
     </button>
     <button
      onClick={() => {
      const entities = form.type === 'homeassistant'
       ? customEntities.filter(ce => ce.entity_id.trim())
       : []
      const configObj: Record<string, any> = {}
      if (form.url) configObj.link_url = form.url
      if (entities.length > 0) configObj.entities = entities
      if (form.type === 'openclash' && sshHost.trim()) {
       configObj.ssh_host = sshHost.trim()
       configObj.ssh_port = parseInt(sshPort) || 22
       configObj.ssh_user = sshUser.trim()
       if (sshPassword) configObj.ssh_password = sshPassword
      }
      const config = Object.keys(configObj).length > 0 ? JSON.stringify(configObj) : ''
      const finalGroup = newGroup.trim() || form.widget_group || ''
      onSave({ ...form, config, widget_group: finalGroup }); onClose()
      setForm({ type: '', name: '', url: '', api_token: '', node: '', description: '', widget_group: '' })
      setNewGroup('')
      setCustomEntities([{ entity_id: '', label: '' }])
      setSshHost('10.0.0.1')
      setSshPort('22')
      setSshUser('root')
      setSshPassword('')
     }}
      className="px-4 py-2 text-sm gradient-bar text-white rounded-lg"
      disabled={!form.name || (!form.url && form.type !== 'hawkeye')}
     >
      添加
     </button>
    </div>
   </div>
  </div>
 )
}

// --- Edit Widget Dialog ---
function EditWidgetDialog({ widget, onClose, onSave, existingGroups }: { widget: Widget; onClose: () => void; onSave: (data: Partial<Widget>) => void; existingGroups: string[] }) {
 const parsedConfig = (() => { try { return JSON.parse(widget.config || '{}') } catch { return {} } })()
 const [form, setForm] = useState<Partial<Widget>>({
  type: widget.type,
  name: widget.name,
  url: widget.url,
  api_token: widget.api_token,
  node: widget.node,
  description: widget.description,
  widget_group: widget.widget_group || '',
 })
 const [linkUrl, setLinkUrl] = useState(parsedConfig.link_url || '')
 const [externalUrl, setExternalUrl] = useState(parsedConfig.external_url || '')
 const [newGroup, setNewGroup] = useState('')
 const [sshHost, setSshHost] = useState(parsedConfig.ssh_host || '10.0.0.1')
 const [sshPort, setSshPort] = useState(String(parsedConfig.ssh_port || '22'))
 const [sshUser, setSshUser] = useState(parsedConfig.ssh_user || 'root')
 const [sshPassword, setSshPassword] = useState(parsedConfig.ssh_password || '')

 return (
  <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/60 backdrop-blur-sm">
   <div className="bg-bg-card/70 rounded-xl p-6 w-full max-w-md shadow-glow">
    <h3 className="text-lg font-bold mb-4">编辑组件 - {widget.name}</h3>

    <div className="space-y-3">
     <div>
      <label className="text-xs text-white block mb-1">名称</label>
      <input
       className="w-full bg-bg-hover border rounded-lg px-3 py-2 text-sm"
       value={form.name || ''}
       onChange={e => setForm({ ...form, name: e.target.value })}
      />
     </div>

     <div>
      <label className="text-xs text-white block mb-1">分组</label>
      <div className="flex gap-2">
       <select
        className="flex-1 bg-bg-hover border rounded-lg px-3 py-2 text-sm"
        value={form.widget_group || ''}
        onChange={e => setForm({ ...form, widget_group: e.target.value })}
       >
        <option value="">未分组</option>
        {existingGroups.map(g => <option key={g} value={g}>{g}</option>)}
       </select>
       <input
        className="flex-1 bg-bg-hover border rounded-lg px-3 py-2 text-sm"
        placeholder="或输入新分组名"
        value={newGroup}
        onChange={e => { setNewGroup(e.target.value); setForm({ ...form, widget_group: e.target.value }) }}
       />
      </div>
     </div>

     <div>
      <label className="text-xs text-white block mb-1">地址</label>
      <input
       className="w-full bg-bg-hover border rounded-lg px-3 py-2 text-sm"
       value={form.url || ''}
       onChange={e => setForm({ ...form, url: e.target.value })}
      />
     </div>

     <div>
      <label className="text-xs text-white block mb-1">API Token</label>
      <input
       className="w-full bg-bg-hover border rounded-lg px-3 py-2 text-sm"
       value={form.api_token || ''}
       onChange={e => setForm({ ...form, api_token: e.target.value })}
      />
     </div>

     {['proxmox', 'pbs', 'openwrt'].includes(widget.type) && (
     <div>
      <label className="text-xs text-white block mb-1">节点/存储名称</label>
      <input
       className="w-full bg-bg-hover border rounded-lg px-3 py-2 text-sm"
       value={form.node || ''}
       onChange={e => setForm({ ...form, node: e.target.value })}
      />
     </div>
     )}

     {widget.type === 'openclash' && (
     <div className="space-y-3 border-t border-white/10 pt-3 mt-3">
      <p className="text-xs text-white/70 font-medium">SSH 控制（用于启停 OpenClash 服务）</p>
      <div>
       <label className="text-xs text-white block mb-1">路由器 IP</label>
       <input className="w-full bg-bg-hover border rounded-lg px-3 py-2 text-sm" placeholder="10.0.0.1" value={sshHost} onChange={e => setSshHost(e.target.value)} />
      </div>
      <div className="flex gap-2">
       <div className="flex-1">
        <label className="text-xs text-white block mb-1">SSH 端口</label>
        <input className="w-full bg-bg-hover border rounded-lg px-3 py-2 text-sm" placeholder="22" value={sshPort} onChange={e => setSshPort(e.target.value)} />
       </div>
       <div className="flex-1">
        <label className="text-xs text-white block mb-1">SSH 用户名</label>
        <input className="w-full bg-bg-hover border rounded-lg px-3 py-2 text-sm" placeholder="root" value={sshUser} onChange={e => setSshUser(e.target.value)} />
       </div>
      </div>
      <div>
       <label className="text-xs text-white block mb-1">SSH 密码</label>
       <input type="password" className="w-full bg-bg-hover border rounded-lg px-3 py-2 text-sm" placeholder="路由器 SSH 密码" value={sshPassword} onChange={e => setSshPassword(e.target.value)} />
      </div>
     </div>
     )}

     <div>
      <label className="text-xs text-white block mb-1">描述（可选）</label>
      <input
       className="w-full bg-bg-hover border rounded-lg px-3 py-2 text-sm"
       placeholder="显示在名称下方的备注信息"
       value={form.description || ''}
       onChange={e => setForm({ ...form, description: e.target.value })}
      />
     </div>

     <div>
      <label className="text-xs text-white block mb-1">内网跳转（可选）</label>
      <input
       className="w-full bg-bg-hover border rounded-lg px-3 py-2 text-sm"
       placeholder="内网点击标题跳转的 URL，如 https://192.168.1.200:8006"
       value={linkUrl}
       onChange={e => setLinkUrl(e.target.value)}
      />
     </div>

     <div>
      <label className="text-xs text-white block mb-1">外网跳转（可选）</label>
      <input
       className="w-full bg-bg-hover border rounded-lg px-3 py-2 text-sm"
       placeholder="外网点击标题跳转的 URL，如 https://your-domain.com:8006"
       value={externalUrl}
       onChange={e => setExternalUrl(e.target.value)}
      />
     </div>
    </div>

    <div className="flex justify-end gap-2 mt-6">
     <button
      onClick={onClose}
      className="px-4 py-2 text-sm text-white hover:text-white transition-colors"
     >
      取消
     </button>
     <button
      onClick={() => {
       const configObj = { ...parsedConfig, link_url: linkUrl || undefined, external_url: externalUrl || undefined }
       if (widget.type === 'openclash') {
        configObj.ssh_host = sshHost.trim()
        configObj.ssh_port = parseInt(sshPort) || 22
        configObj.ssh_user = sshUser.trim()
        if (sshPassword) configObj.ssh_password = sshPassword
        else delete configObj.ssh_password
       }
       const configStr = JSON.stringify(configObj)
       onSave({ ...form, config: configStr === '{}' ? '' : configStr })
       onClose()
      }}
      className="px-4 py-2 text-sm gradient-bar text-white rounded-lg"
      disabled={!form.name}
     >
      保存
     </button>
    </div>
   </div>
  </div>
 )
}

// --- Context Menu ---
function WidgetContextMenu({ x, y, onEdit, onDelete, onClose }: { x: number; y: number; onEdit: () => void; onDelete: () => void; onClose: () => void }) {
 return (
  <>
   <div className="fixed inset-0 z-50" onClick={onClose} onContextMenu={e => { e.preventDefault(); onClose() }} />
   <div
    className="fixed z-50 bg-bg-card/70 rounded-lg shadow-glow py-1 min-w-[120px] animate-in fade-in zoom-in-95 duration-100"
    style={{ left: x, top: y }}
   >
    <button
     onClick={onEdit}
     className="w-full px-3 py-2 text-left text-sm text-white hover:bg-white/5 transition-colors flex items-center gap-2"
    >
     <svg className="w-3.5 h-3.5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
      <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M11 5H6a2 2 0 00-2 2v11a2 2 0 002 2h11a2 2 0 002-2v-5m-1.414-9.414a2 2 0 112.828 2.828L11.828 15H9v-2.828l8.586-8.586z" />
     </svg>
     编辑组件
    </button>
    <button
     onClick={onDelete}
     className="w-full px-3 py-2 text-left text-sm text-white hover:bg-err/10 hover:text-err transition-colors flex items-center gap-2"
    >
     <svg className="w-3.5 h-3.5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
      <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M19 7l-.867 12.142A2 2 0 0116.138 21H7.862a2 2 0 01-1.995-1.858L5 7m5 4v6m4-6v6m1-10V4a1 1 0 00-1-1h-4a1 1 0 00-1 1v3M4 7h16" />
     </svg>
     删除组件
    </button>
   </div>
  </>
 )
}

// --- Empty Group Drop Zone ---
function EmptyGroupDropZone({ groupName }: { groupName: string }) {
 const { setNodeRef, isOver } = useDroppable({ id: `droppable-${groupName}` })
 return (
  <div
   ref={setNodeRef}
   className={`rounded-xl py-12 text-center text-xs transition-colors ${
    isOver ? 'bg-blue-500/10 text-blue-400' : 'bg-bg-hover text-white'
   }`}
  >
   拖拽组件到此处
  </div>
 )
}

// --- Sortable Widget Wrapper ---
function SortableWidget({ widget, children, editMode }: { widget: Widget; children: React.ReactNode; editMode: boolean }) {
 const { attributes, listeners, setNodeRef, transform, transition, isDragging } = useSortable({ id: widget.id })

 const style = {
  transform: CSS.Transform.toString(transform),
  transition,
  opacity: isDragging ? 0.5 : 1,
 }

 if (!editMode) return <>{children}</>

 return (
  <div ref={setNodeRef} style={style} {...attributes} {...listeners} className="cursor-grab active:cursor-grabbing">
   <div className="ring-2 ring-blue-500/30 rounded-xl">
    {children}
   </div>
  </div>
 )
}

// --- Main Page ---
export default function HomepageWidgets() {
 const queryClient = useQueryClient()
 const [showAdd, setShowAdd] = useState(false)
 const [editWidget, setEditWidget] = useState<Widget | null>(null)
 const [contextMenu, setContextMenu] = useState<{ x: number; y: number; widgetId: number } | null>(null)
 const [editMode, setEditMode] = useState(false)
 const [renamingGroup, setRenamingGroup] = useState<string | null>(null)
 const [renameValue, setRenameValue] = useState('')
 const [emptyGroups, setEmptyGroups] = useState<string[]>([])

 const { data: widgetList = [], isLoading } = useQuery({
  queryKey: ['widgets'],
  queryFn: widgets.list,
 })

 const createMutation = useMutation({
  mutationFn: (data: Partial<Widget>) => widgets.create(data),
  onSuccess: () => queryClient.invalidateQueries({ queryKey: ['widgets'] }),
 })

 const deleteMutation = useMutation({
  mutationFn: (id: number) => widgets.delete(id),
  onSuccess: () => queryClient.invalidateQueries({ queryKey: ['widgets'] }),
 })

 const updateMutation = useMutation({
  mutationFn: ({ id, data }: { id: number; data: Partial<Widget> }) => widgets.update(id, data),
  onSuccess: () => queryClient.invalidateQueries({ queryKey: ['widgets'] }),
 })

 const reorderMutation = useMutation({
  mutationFn: (ids: number[]) => widgets.reorder(ids),
  onSuccess: () => queryClient.invalidateQueries({ queryKey: ['widgets'] }),
 })

 const moveMutation = useMutation({
  mutationFn: ({ id, group }: { id: number; group: string }) => widgets.move(id, group),
  onSuccess: () => queryClient.invalidateQueries({ queryKey: ['widgets'] }),
 })

 const renameGroupMutation = useMutation({
  mutationFn: ({ oldName, newName }: { oldName: string; newName: string }) => widgets.renameGroup(oldName, newName),
  onSuccess: () => queryClient.invalidateQueries({ queryKey: ['widgets'] }),
 })

 // Group widgets by widget_group, merge with empty groups
 const { groups, existingGroups } = useMemo(() => {
  const groupMap = new Map<string, Widget[]>()
  // Initialize empty groups first
  for (const g of emptyGroups) groupMap.set(g, [])
  // Then add widgets
  for (const w of widgetList) {
   const g = w.widget_group || '未分组'
   if (!groupMap.has(g)) groupMap.set(g, [])
   groupMap.get(g)!.push(w)
  }
  const groups = Array.from(groupMap.entries())
  const existingGroups = Array.from(new Set(widgetList.map(w => w.widget_group).filter(Boolean)))
  return { groups, existingGroups }
 }, [widgetList, emptyGroups])

 const sensors = useSensors(
  useSensor(PointerSensor, { activationConstraint: { distance: 8 } })
 )

 const handleDragEnd = (event: DragEndEvent) => {
  const { active, over } = event
  if (!over || active.id === over.id) return

  // Handle drop into empty group (over.id is a droppable string)
  if (typeof over.id === 'string' && over.id.startsWith('droppable-')) {
   const targetGroup = over.id.replace('droppable-', '')
   moveMutation.mutate(
    { id: active.id as number, group: targetGroup === '未分组' ? '' : targetGroup },
    {
     onSuccess: () => {
      setEmptyGroups(prev => prev.filter(g => g !== targetGroup))
      reorderMutation.mutate([active.id as number])
     }
    }
   )
   return
  }

  const activeId = active.id as number
  const overId = over.id as number

  const activeWidget = widgetList.find(w => w.id === activeId)
  const overWidget = widgetList.find(w => w.id === overId)
  if (!activeWidget || !overWidget) return

  const activeGroup = activeWidget.widget_group || '未分组'
  const overGroup = overWidget.widget_group || '未分组'

  // Build group map from current data
  const groupMap = new Map<string, Widget[]>()
  for (const w of widgetList) {
   const g = w.widget_group || '未分组'
   if (!groupMap.has(g)) groupMap.set(g, [])
   groupMap.get(g)!.push(w)
  }

  if (activeGroup === overGroup) {
   // Same group: reorder within group
   const groupWidgets = groupMap.get(activeGroup)!
   const oldIndex = groupWidgets.findIndex(w => w.id === activeId)
   const newIndex = groupWidgets.findIndex(w => w.id === overId)
   if (oldIndex === -1 || newIndex === -1) return
   const reordered = [...groupWidgets]
   reordered.splice(oldIndex, 1)
   reordered.splice(newIndex, 0, activeWidget)
   groupMap.set(activeGroup, reordered)

   // Build full order preserving group sequence
   const fullOrder: number[] = []
   for (const [, gWidgets] of groups) {
    const currentIds = (groupMap.get(gWidgets[0]?.widget_group || '未分组') || gWidgets).map(w => w.id)
    fullOrder.push(...currentIds)
   }
   reorderMutation.mutate(fullOrder)
  } else {
   // Cross-group move: update widget_group then reorder
   const activeGroupWidgets = (groupMap.get(activeGroup) || []).filter(w => w.id !== activeId)
   const overGroupWidgets = [...(groupMap.get(overGroup) || [])]
   const overIdx = overGroupWidgets.findIndex(w => w.id === overId)
   if (overIdx === -1) return

   // Insert active widget at over position
   overGroupWidgets.splice(overIdx, 0, { ...activeWidget, widget_group: overGroup })

   groupMap.set(activeGroup, activeGroupWidgets)
   groupMap.set(overGroup, overGroupWidgets)

   // Persist: update widget_group + reorder
   moveMutation.mutate(
    { id: activeId, group: overGroup === '未分组' ? '' : overGroup },
    {
     onSuccess: () => {
      const fullOrder: number[] = []
      // Use original group order to build full list
      const seenGroups = new Set<string>()
      for (const [, gWidgets] of groups) {
       const gName = gWidgets[0]?.widget_group || '未分组'
       if (seenGroups.has(gName)) continue
       seenGroups.add(gName)
       const currentIds = (groupMap.get(gName) || []).map(w => w.id)
       fullOrder.push(...currentIds)
      }
      // Include any new groups not in original order
      for (const [gName, gWidgets] of groupMap) {
       if (!seenGroups.has(gName)) {
        fullOrder.push(...gWidgets.map(w => w.id))
       }
      }
      reorderMutation.mutate(fullOrder)
     }
    }
   )
  }
 }

 const handleContextMenu = (e: React.MouseEvent, widgetId: number) => {
  if (editMode) return
  e.preventDefault()
  setContextMenu({ x: e.clientX, y: e.clientY, widgetId })
 }

 const handleRenameGroup = (groupName: string) => {
  setRenamingGroup(groupName)
  setRenameValue(groupName)
 }

 const handleRenameSubmit = () => {
  if (!renamingGroup || !renameValue) {
   setRenamingGroup(null)
   setRenameValue('')
   return
  }
  if (renameValue === renamingGroup) {
   setRenamingGroup(null)
   setRenameValue('')
   return
  }
  // Check for duplicate group name
  const otherNames = groups.filter(([g]) => g !== renamingGroup).map(([g]) => g)
  if (otherNames.includes(renameValue)) {
   alert(`分组名"${renameValue}" 已存在，请使用其他名称`)
   return
  }
  // Handle empty group rename (frontend only)
  if (emptyGroups.includes(renamingGroup)) {
   setEmptyGroups(prev => prev.map(g => g === renamingGroup ? renameValue : g))
   setRenamingGroup(null)
   setRenameValue('')
   return
  }
  renameGroupMutation.mutate(
   { oldName: renamingGroup, newName: renameValue },
   {
    onSuccess: () => { setRenamingGroup(null); setRenameValue('') },
    onError: (err: any) => alert(err?.message || '重命名失败'),
   }
  )
 }

 // Dynamic grid columns based on number of groups (max 4)
 const colCount = Math.min(groups.length || 1, 4)
 const gridColsClass = colCount === 1
  ? ''
  : colCount === 2
  ? 'lg:grid-cols-2'
  : colCount === 3
  ? 'lg:grid-cols-3'
  : 'lg:grid-cols-4'

 return (
  <div className="py-4 px-2 md:py-6 md:px-3 max-w-[1920px] mx-auto">
   {/* Header */}
   <div className="flex items-center justify-between mb-6">
    <div>
     <h2 className="text-xl md:text-2xl font-bold">服务组件</h2>
     <p className="text-white text-sm mt-1">监控内网服务状态 · 右键管理组件</p>
    </div>
    <div className="flex items-center gap-2">
     <button
      onClick={() => {
       setEditMode(!editMode)
       if (editMode) setEmptyGroups([])
      }}
      className={`px-4 py-2 text-sm rounded-lg transition-all ${editMode ? 'bg-blue-500/20 text-blue-400 border border-blue-500/50' : 'bg-white/5 text-white border hover:text-white'}`}
     >
      {editMode ? '✓ 完成排列' : '⇄ 排列组件'}
     </button>
     {editMode && (
      <button
       onClick={() => {
        const totalGroups = groups.length
        if (totalGroups >= 4) return
        let i = 1
        while (groups.find(([g]) => g === `新组合 ${i}`)) i++
        setEmptyGroups(prev => [...prev, `新组合 ${i}`])
       }}
       disabled={groups.length >= 4}
       className={`px-4 py-2 text-sm rounded-lg transition-all ${
        groups.length >= 4
         ? 'bg-white/5 text-white border cursor-not-allowed'
         : 'bg-green-500/10 text-green-400 border border-green-500/30 hover:bg-green-500/20'
       }`}
       title={groups.length >= 4 ? '最多 4 列' : '新建一列'}
      >
       + 新建组合
      </button>
     )}
     <button
      onClick={() => setShowAdd(true)}
      className="px-4 py-2 text-sm gradient-bar text-white rounded-lg hover:opacity-90 transition-opacity"
     >
      + 添加组件
     </button>
    </div>
   </div>

   {/* Widget Grid - Grouped Layout */}
   {isLoading ? (
    <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-4 gap-4 items-start">
     {[1, 2, 3, 4].map(i => <WidgetSkeleton key={i} name="加载中..." />)}
    </div>
   ) : widgetList.length === 0 ? (
    <div className="text-center py-20">
     <p className="text-white text-lg mb-2">还没有添加任何服务组件</p>
     <p className="text-white text-sm mb-4">点击「添加组件」来监控你的 PVE、PBS、Unraid 等服务</p>
     <button
      onClick={() => setShowAdd(true)}
      className="px-4 py-2 text-sm gradient-bar text-white rounded-lg"
     >
      + 添加第一个组件
     </button>
    </div>
   ) : (
    <DndContext sensors={sensors} collisionDetection={closestCorners} onDragEnd={handleDragEnd}>
     <div className={`grid grid-cols-1 md:grid-cols-2 ${gridColsClass} gap-6 items-start`}>
      {groups.map(([groupName, groupWidgets]) => {
       const isEmpty = groupWidgets.length === 0
       if (isEmpty && !editMode) return null
       return (
       <div key={groupName} className="space-y-3">
        {/* Group Title */}
        {editMode && renamingGroup === groupName ? (
         <div className="flex items-center gap-2 pb-2lue-500/50">
          <input
           className="flex-1 text-sm font-bold text-blue-400 uppercase tracking-wider px-1 bg-transparent outline-none"
           value={renameValue}
           onChange={e => setRenameValue(e.target.value)}
           onBlur={handleRenameSubmit}
           onKeyDown={e => { if (e.key === 'Enter') handleRenameSubmit(); if (e.key === 'Escape') { setRenamingGroup(null); setRenameValue('') } }}
           autoFocus
          />
          {emptyGroups.includes(groupName) && (
           <button
            onClick={() => setEmptyGroups(prev => prev.filter(g => g !== groupName))}
            className="text-red-400 hover:text-red-300 text-xs px-1"
            title="删除空组合"
           >
            ✕
           </button>
          )}
         </div>
        ) : (
         <div className="flex items-center pb-2">
          <h3
           className={`text-sm font-bold text-white uppercase tracking-wider px-1 flex-1 ${editMode ? 'cursor-pointer hover:text-blue-400 hover:border-blue-500/50 transition-colors' : ''}`}
           onClick={() => { if (editMode) handleRenameGroup(groupName) }}
           title={editMode ? '点击修改分组名称' : undefined}
          >
           {groupName}
           {editMode && <span className="ml-2 text-xs text-white">✎</span>}
          </h3>
          {editMode && emptyGroups.includes(groupName) && (
           <button
            onClick={() => setEmptyGroups(prev => prev.filter(g => g !== groupName))}
            className="text-red-400 hover:text-red-300 text-xs px-1"
            title="删除空组合"
           >
            ✕
           </button>
          )}
         </div>
        )}

        {/* Widgets or empty drop zone */}
        {groupWidgets.length > 0 ? (
         <SortableContext items={groupWidgets.map(w => w.id)} strategy={verticalListSortingStrategy}>
          <div className="space-y-3">
           {groupWidgets.map(w => (
            <SortableWidget key={w.id} widget={w} editMode={editMode}>
             <div onContextMenu={e => handleContextMenu(e, w.id)}>
              {renderWidget(w)}
             </div>
            </SortableWidget>
           ))}
          </div>
         </SortableContext>
        ) : editMode ? (
         <EmptyGroupDropZone groupName={groupName} />
        ) : null}
       </div>
       )
      })}
     </div>
    </DndContext>
   )}

   {/* Context Menu */}
   {contextMenu && (
    <WidgetContextMenu
     x={contextMenu.x}
     y={contextMenu.y}
     onEdit={() => {
      const w = widgetList.find(w => w.id === contextMenu.widgetId)
      if (w) setEditWidget(w)
      setContextMenu(null)
     }}
     onDelete={() => {
      deleteMutation.mutate(contextMenu.widgetId)
      setContextMenu(null)
     }}
     onClose={() => setContextMenu(null)}
    />
   )}

   {/* Add Dialog */}
   <AddWidgetDialog
    open={showAdd}
    onClose={() => setShowAdd(false)}
    onSave={(data) => createMutation.mutate(data)}
    existingGroups={existingGroups}
   />

   {/* Edit Dialog */}
   {editWidget && (
    <EditWidgetDialog
     widget={editWidget}
     onClose={() => setEditWidget(null)}
     onSave={(data) => {
      updateMutation.mutate({ id: editWidget.id, data })
      setEditWidget(null)
     }}
     existingGroups={existingGroups}
    />
   )}
  </div>
 )
}
