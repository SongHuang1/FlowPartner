import { useState, useEffect, useCallback } from 'react'
import { X, Plus, Trash2 } from 'lucide-react'
import { cn } from '@/lib/utils'
import { Button } from '@/components/ui/button'
import { getHistoryList, getHistorySession, deleteHistory } from '@/lib/api'
import { buildHistoryContentBlocks, buildToolResultMap } from '@/lib/history'
import type { HistoryEntry, Message } from '@/types'

function DeleteConfirmDialog({ onConfirm, onCancel }: { onConfirm: () => void; onCancel: () => void }) {
  useEffect(() => {
    const handleEsc = (e: KeyboardEvent) => { if (e.key === 'Escape') onCancel() }
    window.addEventListener('keydown', handleEsc)
    return () => window.removeEventListener('keydown', handleEsc)
  }, [onCancel])

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/40" onClick={onCancel}>
      <div className="bg-white rounded-xl shadow-2xl w-80 flex flex-col overflow-hidden" onClick={e => e.stopPropagation()}>
        <div className="flex items-center justify-between px-5 py-4 border-b border-neutral-100">
          <h3 className="text-sm font-semibold text-neutral-800">删除对话</h3>
          <button
            onClick={onCancel}
            className="w-7 h-7 rounded-full flex items-center justify-center text-neutral-400 hover:text-neutral-600 hover:bg-neutral-100 transition-colors"
            aria-label="关闭"
          >
            <X className="w-4 h-4" />
          </button>
        </div>
        <div className="p-5">
          <p className="text-sm text-neutral-600 mb-4">确定要删除这条对话记录吗？删除后无法恢复。</p>
          <div className="flex gap-2 justify-end">
            <button
              onClick={onConfirm}
              className="px-4 py-2 text-sm font-medium text-white bg-red-500 rounded-lg hover:bg-red-600 transition-colors"
            >
              确认删除
            </button>
            <button
              onClick={onCancel}
              className="px-4 py-2 text-sm font-medium text-neutral-600 bg-neutral-100 rounded-lg hover:bg-neutral-200 transition-colors"
            >
              取消
            </button>
          </div>
        </div>
      </div>
    </div>
  )
}

interface SidebarProps {
  visible: boolean
  onClose: () => void
  onNewChat: () => void
  onLoadSession: (sessionId: string, messages: Message[]) => void
  refreshTrigger?: number
}

export function Sidebar({ visible, onClose, onNewChat, onLoadSession, refreshTrigger }: SidebarProps) {
  const [historyList, setHistoryList] = useState<HistoryEntry[]>([])
  // 挂载时即开始加载历史，初始状态直接为加载中，避免 effect 内同步 setState。
  const [historyLoading, setHistoryLoading] = useState(true)
  const [historyError, setHistoryError] = useState<string | null>(null)
  const [deletingId, setDeletingId] = useState<string | null>(null)
  const [deleteConfirmId, setDeleteConfirmId] = useState<string | null>(null)

  const loadHistory = useCallback(async () => {
    try {
      const list = await getHistoryList()
      setHistoryList(list)
      setHistoryError(null)
    } catch (e) {
      setHistoryError(e instanceof Error ? e.message : '加载历史失败')
    } finally {
      setHistoryLoading(false)
    }
  }, [])

  useEffect(() => {
    // 通过 timer 回调触发首次加载，避免 effect 同步链路中的 setState 级联渲染。
    const timer = setTimeout(loadHistory, 0)
    return () => clearTimeout(timer)
  }, [loadHistory])

  useEffect(() => {
    if (refreshTrigger && refreshTrigger > 0) {
      loadHistory()
    }
  }, [refreshTrigger, loadHistory])

  const handleLoadSession = async (sessionId: string) => {
    setHistoryLoading(true)
    setHistoryError(null)
    try {
      const session = await getHistorySession(sessionId)
      const subagents = session.subagents || {}
      const toolResults = buildToolResultMap(session.messages)
      const msgs: Message[] = session.messages
        .filter((m) => m.role === 'user' || m.role === 'assistant')
        .map((m, i) => ({
          id: `msg_${sessionId}_${i}`,
          role: m.role as 'user' | 'assistant',
          content: m.content,
          timestamp: Date.now(),
          content_blocks:
            m.role === 'assistant' ? buildHistoryContentBlocks(m, subagents, toolResults) : undefined,
        }))
      onLoadSession(sessionId, msgs)
    } catch (e) {
      setHistoryError(e instanceof Error ? e.message : '加载对话失败')
    } finally {
      setHistoryLoading(false)
    }
  }

  const handleDelete = (sessionId: string, e: React.MouseEvent) => {
    e.stopPropagation()
    setDeleteConfirmId(sessionId)
  }

  const handleDeleteConfirm = async () => {
    const sessionId = deleteConfirmId
    setDeleteConfirmId(null)
    if (!sessionId) return
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

             {!historyLoading && !historyError && historyList.length > 0 && (
               <Button variant="ghost" className="justify-start text-xs text-neutral-400 mt-1" onClick={loadHistory}>
                 刷新列表
               </Button>
             )}
           </div>
         </div>
      </div>
      {deleteConfirmId && (
        <DeleteConfirmDialog
          onConfirm={handleDeleteConfirm}
          onCancel={() => setDeleteConfirmId(null)}
        />
      )}
    </div>
  )
}
