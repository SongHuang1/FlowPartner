import { ChevronRight, Loader2, CheckCircle2, XCircle } from 'lucide-react'
import type { SubAgentRun } from '@/types'

interface SubAgentCardProps {
  run: SubAgentRun
  onClick: (run: SubAgentRun) => void
}

const statusLabel: Record<SubAgentRun['status'], string> = {
  running: '运行中',
  done: '已完成',
  error: '失败',
}

export function SubAgentCard({ run, onClick }: SubAgentCardProps) {
  return (
    <button
      type="button"
      onClick={() => onClick(run)}
      className="w-full flex items-center gap-3 rounded-lg border border-neutral-200 bg-neutral-50 hover:bg-neutral-100 hover:border-blue-300 transition-colors px-4 py-3 text-left"
      aria-label={`查看子智能体 ${run.agent_name} 的执行过程`}
    >
      <div className="flex-1 min-w-0">
        <div className="flex items-center gap-2">
          <span className="text-sm font-medium text-neutral-800">{run.agent_name}</span>
          <span className="text-xs text-neutral-400">层级 {run.depth}</span>
        </div>
        <div className="text-xs text-neutral-500 truncate mt-0.5">
          {run.status === 'running' ? (run.task || '执行中...') : (run.result || run.error || '')}
        </div>
      </div>
      <div className="flex items-center gap-2 shrink-0">
        <span
          className={`inline-flex items-center gap-1 text-xs font-medium ${
            run.status === 'running' ? 'text-blue-600' : run.status === 'done' ? 'text-green-600' : 'text-red-500'
          }`}
        >
          {run.status === 'running' ? (
            <Loader2 className="w-3.5 h-3.5 animate-spin" />
          ) : run.status === 'done' ? (
            <CheckCircle2 className="w-3.5 h-3.5" />
          ) : (
            <XCircle className="w-3.5 h-3.5" />
          )}
          {statusLabel[run.status]}
        </span>
        <ChevronRight className="w-4 h-4 text-neutral-400" />
      </div>
    </button>
  )
}