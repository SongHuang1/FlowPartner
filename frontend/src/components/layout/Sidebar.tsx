import { useState } from 'react'
import { X, Plus, History, ArrowLeft } from 'lucide-react'
import { cn } from '@/lib/utils'
import { Button } from '@/components/ui/button'
import { getHistoryList, getHistorySession } from '@/lib/api'
import type { HistoryEntry, Message } from '@/types'

interface SidebarProps {
  visible: boolean
  onClose: () => void
  onNewChat: () => void
  onLoadSession: (sessionId: string, messages: Message[]) => void
}

export function Sidebar({ visible, onClose, onNewChat, onLoadSession }: SidebarProps) {
  const [view, setView] = useState<'welcome' | 'history'>('welcome')
  const [historyList, setHistoryList] = useState<HistoryEntry[]>([])
  const [historyLoading, setHistoryLoading] = useState(false)
  const [historyError, setHistoryError] = useState<string | null>(null)

  const loadHistory = async () => {
    setHistoryLoading(true)
    setHistoryError(null)
    try {
      const list = await getHistoryList()
      setHistoryList(list)
      setView('history')
    } catch (e) {
      setHistoryError(e instanceof Error ? e.message : '加载历史失败')
    } finally {
      setHistoryLoading(false)
    }
  }

  const handleNewChat = () => {
    onNewChat()
    setView('welcome')
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
        }))
      onLoadSession(sessionId, msgs)
      setView('welcome')
    } catch (e) {
      setHistoryError(e instanceof Error ? e.message : '加载对话失败')
    } finally {
      setHistoryLoading(false)
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
        <div className="flex-1 overflow-y-auto p-4">
          {view === 'welcome' ? (
            <div className="flex flex-col gap-4">
              <h2 className="font-semibold text-base text-neutral-800">欢迎使用 FlowPartner</h2>
              <p className="text-sm text-neutral-600">开始新对话或继续之前的对话</p>
              <div className="flex flex-col gap-2">
                <h3 className="text-xs font-medium text-neutral-500 uppercase tracking-wide">建议操作</h3>
                <Button variant="outline" className="justify-start text-sm" onClick={handleNewChat}>
                  <Plus className="w-4 h-4 mr-2" />
                  开始新对话
                </Button>
                <Button variant="outline" className="justify-start text-sm" onClick={loadHistory}>
                  <History className="w-4 h-4 mr-2" />
                  查看历史
                </Button>
              </div>
            </div>
          ) : (
            <div className="flex flex-col gap-2">
              <div className="flex items-center gap-2 mb-2">
                <Button variant="ghost" size="sm" className="px-2" onClick={() => setView('welcome')} aria-label="返回欢迎页">
                  <ArrowLeft className="w-4 h-4" />
                </Button>
                <h3 className="text-xs font-medium text-neutral-500 uppercase tracking-wide">历史对话</h3>
              </div>
              {historyLoading && <div className="text-sm text-neutral-400">加载中...</div>}
              {historyError && <div className="text-sm text-red-500">{historyError}</div>}
              {!historyLoading && !historyError && historyList.length === 0 && (
                <div className="text-sm text-neutral-400">暂无历史对话</div>
              )}
              {historyList.map((entry) => (
                <button
                  key={entry.session_id}
                  type="button"
                  className="text-left text-sm p-2 rounded-md hover:bg-neutral-100 border border-neutral-200"
                  onClick={() => handleLoadSession(entry.session_id)}
                >
                  <div className="font-medium text-neutral-800 truncate">{entry.title || '未命名对话'}</div>
                  <div className="text-xs text-neutral-400">
                    {new Date(entry.updated_at).toLocaleString()} · {entry.message_count} 条消息
                  </div>
                </button>
              ))}
            </div>
          )}
        </div>
      </div>
    </div>
  )
}