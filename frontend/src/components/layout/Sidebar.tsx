import { useState } from 'react'
import { X, Plus, Trash2 } from 'lucide-react'
import { cn } from '@/lib/utils'
import { Button } from '@/components/ui/button'
import { getHistoryList, getHistorySession, deleteHistory } from '@/lib/api'
import type { HistoryEntry, Message } from '@/types'

interface SidebarProps {
  visible: boolean
  onClose: () => void
  onNewChat: () => void
  onLoadSession: (sessionId: string, messages: Message[]) => void
}

export function Sidebar({ visible, onClose, onNewChat, onLoadSession }: SidebarProps) {
  const [historyList, setHistoryList] = useState<HistoryEntry[]>([])
  const [historyLoading, setHistoryLoading] = useState(false)
  const [historyError, setHistoryError] = useState<string | null>(null)
  const [deletingId, setDeletingId] = useState<string | null>(null)

  const loadHistory = async () => {
    setHistoryLoading(true)
    setHistoryError(null)
    try {
      const list = await getHistoryList()
      setHistoryList(list)
    } catch (e) {
      setHistoryError(e instanceof Error ? e.message : '加载历史失败')
    } finally {
      setHistoryLoading(false)
    }
  }

  const handleLoadSession = async (sessionId: string) => {
    setHistoryLoading(true)
    setHistoryError(null)
    try {
      const session = await getHistorySession(sessionId)
      const msgs: Message[] = session.messages
        .filter((m) => m.role === 'user' || m.role === 'assistant')
        .map((m, i) => ({
          id: `msg_${sessionId}_${i}`,
          role: m.role as 'user' | 'assistant',
          content: m.content,
          timestamp: Date.now(),
          subagent_results: m.subagent_results,
        }))
      onLoadSession(sessionId, msgs)
    } catch (e) {
      setHistoryError(e instanceof Error ? e.message : '加载对话失败')
    } finally {
      setHistoryLoading(false)
    }
  }

  const handleDelete = async (sessionId: string, e: React.MouseEvent) => {
    e.stopPropagation()
    if (!confirm('确定要删除这条对话记录吗？')) return
    setDeletingId(sessionId)
    try {
      await deleteHistory(sessionId)
      setHistoryList((prev) => prev.filter((item) => item.session_id !== sessionId))
    } catch {
      setHistoryError('删除失败')
    } finally {
      setDeletingId(null)
    }
  }

  return (
    <div
      data-testid="sidebar-panel"
      className={cn(
        "border-r border-neutral-200 bg-white flex flex-col shrink-0 overflow-hidden transition-all duration-200",
        visible ? "w-64" : "w-0"
      )}
      aria-hidden={!visible}
    >
      <div className="w-64 flex flex-col h-full">
        <div className="flex items-center justify-between p-3 border-b border-neutral-100">
          <span className="text-sm font-medium text-neutral-700">聊天记录</span>
          <Button variant="ghost" size="icon" className="w-7 h-7" onClick={onClose} aria-label="收起侧栏">
            <X className="w-4 h-4" />
          </Button>
        </div>
        <div className="flex-1 overflow-y-auto p-3">
          <div className="flex flex-col gap-2">
            <div className="mb-2">
              <h2 className="font-semibold text-sm text-neutral-800">欢迎使用 FlowPartner</h2>
              <p className="text-xs text-neutral-500 mt-0.5">开始新对话或继续之前的对话</p>
            </div>

            <Button variant="outline" className="justify-start text-sm" onClick={() => { onNewChat() }}>
              <Plus className="w-4 h-4 mr-2" />
              开始新对话
            </Button>

            {historyLoading && historyList.length === 0 && (
              <div className="text-sm text-neutral-400 px-2 py-1">加载中...</div>
            )}
            {historyError && (
              <div className="text-sm text-red-500 px-2 py-1">{historyError}</div>
            )}

            {historyList.length > 0 && (
              <h3 className="text-xs font-medium text-neutral-500 uppercase tracking-wide mt-2">历史对话</h3>
            )}

            {historyList.map((entry) => (
              <div
                key={entry.session_id}
                className={cn(
                  "group flex items-center gap-1 rounded-md border border-neutral-200 hover:bg-neutral-50 transition-colors",
                  deletingId === entry.session_id && "opacity-50"
                )}
              >
                <button
                  type="button"
                  className="flex-1 text-left text-sm p-2 min-w-0"
                  onClick={() => handleLoadSession(entry.session_id)}
                >
                  <div className="font-medium text-neutral-800 truncate">{entry.title || '未命名对话'}</div>
                  <div className="text-xs text-neutral-400">
                    {new Date(entry.updated_at).toLocaleDateString()} · {entry.message_count} 条
                  </div>
                </button>
                <button
                  type="button"
                  className="p-1.5 mr-1 rounded opacity-0 group-hover:opacity-100 hover:bg-neutral-200 transition-opacity shrink-0"
                  onClick={(e) => handleDelete(entry.session_id, e)}
                  aria-label="删除对话"
                  disabled={deletingId === entry.session_id}
                >
                  <Trash2 className="w-3.5 h-3.5 text-neutral-400 hover:text-red-500" />
                </button>
              </div>
            ))}

            {!historyLoading && !historyError && historyList.length === 0 && (
              <Button variant="ghost" className="justify-start text-sm text-neutral-500" onClick={loadHistory}>
                查看历史
              </Button>
            )}
            {!historyLoading && !historyError && historyList.length > 0 && (
              <Button variant="ghost" className="justify-start text-xs text-neutral-400 mt-1" onClick={loadHistory}>
                刷新列表
              </Button>
            )}
          </div>
        </div>
      </div>
    </div>
  )
}
