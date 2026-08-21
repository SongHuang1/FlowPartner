import { useCallback, useEffect, useState } from 'react'
import { Plus, Pencil, Trash2, X, Loader2 } from 'lucide-react'
import { Button } from '@/components/ui/button'
import { createAgent, deleteAgent, getAgent, listAgents, updateAgent } from '@/lib/api'
import type { AgentDef, AgentMeta } from '@/types'

const textareaClass = 'flex w-full rounded-lg border border-neutral-200 bg-white px-4 py-3 text-sm shadow-sm placeholder:text-neutral-400 focus:outline-none focus:ring-2 focus:ring-blue-500/20 focus:border-blue-400 resize-none'
const inputClass = 'flex w-full rounded-lg border border-neutral-200 bg-white px-4 py-2.5 text-sm shadow-sm placeholder:text-neutral-400 focus:outline-none focus:ring-2 focus:ring-blue-500/20 focus:border-blue-400'

interface FormState {
  name: string
  description: string
  system_prompt: string
}

const emptyForm: FormState = { name: '', description: '', system_prompt: '' }

export function AgentsManager() {
  const [agents, setAgents] = useState<AgentMeta[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)
  const [editing, setEditing] = useState<AgentDef | null>(null)
  const [creating, setCreating] = useState(false)
  const [form, setForm] = useState<FormState>(emptyForm)
  const [saving, setSaving] = useState(false)
  const [deletingId, setDeletingId] = useState<string | null>(null)

  const refresh = useCallback(async () => {
    try {
      const items = await listAgents()
      setAgents(items)
      setError(null)
    } catch (e) {
      setError(`加载智能体列表失败：${e instanceof Error ? e.message : String(e)}`)
    } finally {
      setLoading(false)
    }
  }, [])

  useEffect(() => {
    ;(async () => {
      await refresh()
    })()
  }, [refresh])

  const startCreate = () => {
    setCreating(true)
    setEditing(null)
    setForm(emptyForm)
  }

  const startEdit = async (agent: AgentMeta) => {
    try {
      const detail = await getAgent(agent.id)
      setEditing(detail)
      setCreating(false)
      setForm({
        name: detail.name,
        description: detail.description,
        system_prompt: detail.system_prompt,
      })
      setError(null)
    } catch (e) {
      setError(`加载智能体详情失败：${e instanceof Error ? e.message : String(e)}`)
    }
  }

  const cancelEdit = () => {
    setCreating(false)
    setEditing(null)
    setForm(emptyForm)
  }

  const handleSave = async () => {
    if (!form.name.trim() || !form.description.trim() || !form.system_prompt.trim()) {
      setError('名称、对外描述、系统提示词均为必填项')
      return
    }
    setSaving(true)
    setError(null)
    try {
      if (editing) {
        await updateAgent(editing.id, form)
      } else {
        await createAgent(form)
      }
      cancelEdit()
      await refresh()
    } catch (e) {
      setError(`保存失败：${e instanceof Error ? e.message : String(e)}`)
    } finally {
      setSaving(false)
    }
  }

  const handleDelete = async (agent: AgentMeta) => {
    if (!window.confirm(`确定删除智能体「${agent.name}」？删除后无法恢复。`)) return
    setDeletingId(agent.id)
    setError(null)
    try {
      await deleteAgent(agent.id)
      await refresh()
    } catch (e) {
      setError(`删除失败：${e instanceof Error ? e.message : String(e)}`)
    } finally {
      setDeletingId(null)
    }
  }

  return (
    <div className="flex flex-col gap-4">
      <div className="flex items-center justify-between">
        <h4 className="text-sm font-medium text-neutral-700">智能体列表</h4>
        <Button size="sm" onClick={startCreate} disabled={creating || !!editing}>
          <Plus className="w-4 h-4 mr-1" />
          新建智能体
        </Button>
      </div>

      {error && (
        <div className="p-3 rounded-lg bg-red-50 border border-red-100 text-sm text-red-600">
          {error}
        </div>
      )}

      {loading ? (
        <div className="flex items-center gap-2 text-sm text-neutral-400 py-4">
          <Loader2 className="w-4 h-4 animate-spin" />
          加载中...
        </div>
      ) : agents.length === 0 ? (
        <p className="text-sm text-neutral-400 py-2">暂无智能体。点击「新建智能体」创建一个专职智能体，之后可在会话中调用它。</p>
      ) : (
        <ul className="flex flex-col gap-2">
          {agents.map((agent) => (
            <li key={agent.id} className="flex items-center gap-3 p-3 rounded-lg border border-neutral-200 bg-neutral-50">
              <div className="flex-1 min-w-0">
                <div className="flex items-center gap-2">
                  <span className="text-sm font-medium text-neutral-800">{agent.name}</span>
                  {agent.id === 'main' && (
                    <span className="text-[10px] px-1.5 py-0.5 rounded bg-blue-50 text-blue-600 font-medium">内置</span>
                  )}
                </div>
                <p className="text-xs text-neutral-500 truncate mt-0.5">{agent.description}</p>
              </div>
              <div className="flex items-center gap-1 shrink-0">
                <Button
                  variant="ghost"
                  size="icon"
                  className="w-8 h-8 rounded-lg"
                  onClick={() => startEdit(agent)}
                  aria-label={`编辑 ${agent.name}`}
                >
                  <Pencil className="w-4 h-4 text-neutral-500" />
                </Button>
                {agent.id !== 'main' && (
                  <Button
                    variant="ghost"
                    size="icon"
                    className="w-8 h-8 rounded-lg hover:bg-red-50"
                    onClick={() => handleDelete(agent)}
                    disabled={deletingId === agent.id}
                    aria-label={`删除 ${agent.name}`}
                  >
                    {deletingId === agent.id ? (
                      <Loader2 className="w-4 h-4 text-red-400 animate-spin" />
                    ) : (
                      <Trash2 className="w-4 h-4 text-red-400" />
                    )}
                  </Button>
                )}
              </div>
            </li>
          ))}
        </ul>
      )}

      {(creating || editing) && (
        <div className="flex flex-col gap-3 p-4 rounded-lg border border-blue-200 bg-blue-50/40">
          <div className="flex items-center justify-between">
            <h4 className="text-sm font-medium text-neutral-700">
              {editing ? `编辑智能体「${editing.name}」` : '新建智能体'}
            </h4>
            <Button variant="ghost" size="icon" className="w-7 h-7 rounded-full" onClick={cancelEdit} aria-label="取消编辑">
              <X className="w-4 h-4" />
            </Button>
          </div>
          <div className="flex flex-col gap-1.5">
            <label htmlFor="agent-name" className="text-xs font-medium text-neutral-600">名称</label>
            <input
              id="agent-name"
              className={inputClass}
              value={form.name}
              onChange={(e) => setForm({ ...form, name: e.target.value })}
              placeholder="如：翻译官"
              maxLength={128}
            />
          </div>
          <div className="flex flex-col gap-1.5">
            <label htmlFor="agent-description" className="text-xs font-medium text-neutral-600">对外描述</label>
            <textarea
              id="agent-description"
              className={textareaClass}
              rows={3}
              value={form.description}
              onChange={(e) => setForm({ ...form, description: e.target.value })}
              placeholder="向其他智能体介绍你的职责与专长（用于它们决定是否调用你）"
              maxLength={2000}
            />
          </div>
          <div className="flex flex-col gap-1.5">
            <label htmlFor="agent-prompt" className="text-xs font-medium text-neutral-600">系统提示词（仅编辑时可见）</label>
            <textarea
              id="agent-prompt"
              className={textareaClass}
              rows={6}
              value={form.system_prompt}
              onChange={(e) => setForm({ ...form, system_prompt: e.target.value })}
              placeholder="详细定义该智能体的行为准则、技能与限制"
              maxLength={200000}
            />
            <p className="text-xs text-neutral-400">提示词为私有内容，不会出现在列表、对话或日志中</p>
          </div>
          <div className="flex justify-end gap-2">
            <Button variant="outline" size="sm" onClick={cancelEdit} disabled={saving}>
              取消
            </Button>
            <Button size="sm" onClick={handleSave} disabled={saving}>
              {saving ? '保存中...' : '保存'}
            </Button>
          </div>
        </div>
      )}
    </div>
  )
}