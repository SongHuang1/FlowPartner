import { X, Loader2, CheckCircle2, XCircle } from 'lucide-react'
import { Button } from '@/components/ui/button'
import type { SubAgentRun, SubAgentStep } from '@/types'

interface SubAgentDrilldownProps {
  run: SubAgentRun
  onClose: () => void
}

function StepRow({ step }: { step: SubAgentStep }) {
  switch (step.step_type) {
    case 'thinking':
      return (
        <div className="flex gap-2">
          <span className="text-xs text-neutral-400 mt-0.5 shrink-0">思考</span>
          <p className="text-sm text-neutral-700 whitespace-pre-wrap">{step.content}</p>
        </div>
      )
    case 'tool_call':
      return (
        <div className="flex gap-2">
          <span className="text-xs text-blue-500 mt-0.5 shrink-0">行动</span>
          <p className="text-sm text-neutral-700">
            调用工具 <code className="px-1 py-0.5 rounded bg-neutral-100 text-xs">{step.tool}</code>
            {step.args ? `：${JSON.stringify(step.args)}` : ''}
          </p>
        </div>
      )
    case 'tool_result':
      return (
        <div className="flex gap-2">
          <span className="text-xs text-green-600 mt-0.5 shrink-0">观察</span>
          <p className="text-sm text-neutral-600 whitespace-pre-wrap">
            {step.result}
            {step.truncated ? '（结果过长已截断）' : ''}
          </p>
        </div>
      )
    case 'final_answer':
      return (
        <div className="flex gap-2">
          <span className="text-xs text-neutral-400 mt-0.5 shrink-0">结论</span>
          <p className="text-sm text-neutral-800 whitespace-pre-wrap">{step.content}</p>
        </div>
      )
  }
}

export function SubAgentDrilldown({ run, onClose }: SubAgentDrilldownProps) {
  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/40" role="dialog" aria-label={`子智能体 ${run.agent_name} 执行详情`}>
      <div className="bg-white rounded-xl shadow-2xl w-[640px] max-h-[80vh] flex flex-col overflow-hidden">
        <div className="flex items-center justify-between px-6 py-4 border-b border-neutral-200 bg-neutral-50">
          <div className="flex items-center gap-3">
            <div>
              <h3 className="text-base font-semibold text-neutral-800">{run.agent_name}</h3>
              <p className="text-xs text-neutral-400">
                层级 {run.depth} · trace {run.trace_id.slice(0, 8)}… · span {run.span_id.slice(0, 8)}…
              </p>
            </div>
          </div>
          <div className="flex items-center gap-2">
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
              {run.status === 'running' ? '运行中' : run.status === 'done' ? '已完成' : '失败'}
            </span>
            <Button variant="ghost" size="icon" className="w-8 h-8 rounded-full" onClick={onClose} aria-label="退出">
              <X className="w-4 h-4" />
            </Button>
          </div>
        </div>

        <div className="flex-1 overflow-y-auto p-6 space-y-4">
          <div>
            <h4 className="text-xs font-medium text-neutral-500 mb-1.5">任务</h4>
            <p className="text-sm text-neutral-700 whitespace-pre-wrap">{run.task || '（无）'}</p>
          </div>

          {run.error && (
            <div className="p-3 rounded-lg bg-red-50 border border-red-100 text-sm text-red-600">
              {run.error}
            </div>
          )}

          <div>
            <h4 className="text-xs font-medium text-neutral-500 mb-2">执行过程</h4>
            {run.steps.length === 0 ? (
              <p className="text-sm text-neutral-400">暂无步骤</p>
            ) : (
              <div className="flex flex-col gap-3 pl-2 border-l-2 border-neutral-100">
                {run.steps.map((step, i) => (
                  <StepRow key={i} step={step} />
                ))}
              </div>
            )}
          </div>

          {run.result !== undefined && (
            <div>
              <h4 className="text-xs font-medium text-neutral-500 mb-1.5">最终结果</h4>
              <p className="text-sm text-neutral-800 whitespace-pre-wrap bg-neutral-50 rounded-lg p-3 border border-neutral-100">
                {run.result}
              </p>
            </div>
          )}
        </div>

        <div className="flex justify-end px-6 py-4 border-t border-neutral-200 bg-neutral-50">
          <Button onClick={onClose}>返回主会话</Button>
        </div>
      </div>
    </div>
  )
}