import { useState } from 'react'
import { ChevronDown, ChevronRight, Wrench, AlertTriangle } from 'lucide-react'
import type { IterationStep } from '@/types'

interface IterationStepViewProps {
  step: IterationStep
  isLast: boolean
}

function ToolCallBlock({ tc }: { tc: IterationStep['toolCalls'][number] }) {
  const [expanded, setExpanded] = useState(false)

  return (
    <div className="border border-neutral-200 rounded-md text-sm">
      <button
        onClick={() => setExpanded(!expanded)}
        className="w-full flex items-center gap-2 px-3 py-1.5 text-left hover:bg-neutral-50"
      >
        {expanded ? (
          <ChevronDown className="w-3.5 h-3.5 text-neutral-400" />
        ) : (
          <ChevronRight className="w-3.5 h-3.5 text-neutral-400" />
        )}
        <Wrench className="w-3.5 h-3.5 text-blue-500" />
        <span className="font-mono text-blue-600">{tc.tool}</span>
        <span className="text-neutral-400 text-xs">工具调用</span>
      </button>
      {expanded && (
        <div className="px-3 pb-2 pt-1 border-t border-neutral-100 space-y-1.5">
          <div>
            <span className="text-xs font-medium text-neutral-500">参数</span>
            <pre className="text-xs text-neutral-600 overflow-x-auto whitespace-pre-wrap mt-0.5">
              {JSON.stringify(tc.args, null, 2)}
            </pre>
          </div>
          {tc.result !== undefined && (
            <div>
              <div className="flex items-center gap-1.5">
                <span className="text-xs font-medium text-neutral-500">结果</span>
                {tc.truncated && (
                  <span className="text-xs text-amber-600 bg-amber-50 px-1.5 py-0.5 rounded">
                    已截断
                  </span>
                )}
              </div>
              <pre className="text-xs text-neutral-600 overflow-x-auto whitespace-pre-wrap mt-0.5 max-h-60 overflow-y-auto">
                {tc.result}
              </pre>
            </div>
          )}
        </div>
      )}
    </div>
  )
}

export function IterationStepView({ step, isLast }: IterationStepViewProps) {
  const [expanded, setExpanded] = useState(isLast)
  const hasToolCalls = step.toolCalls.length > 0
  const hasTerminated = !!step.loopTerminated

  return (
    <div className="border border-neutral-200 rounded-lg overflow-hidden">
      <button
        onClick={() => setExpanded(!expanded)}
        className="w-full flex items-center gap-2 px-3 py-2 text-left hover:bg-neutral-50 bg-neutral-50/50"
      >
        {expanded ? (
          <ChevronDown className="w-4 h-4 text-neutral-400" />
        ) : (
          <ChevronRight className="w-4 h-4 text-neutral-400" />
        )}
        <span className="text-xs font-semibold text-neutral-600">第 {step.iteration} 轮</span>
        {hasToolCalls && (
          <span className="text-xs text-blue-600 bg-blue-50 px-1.5 py-0.5 rounded">
            {step.toolCalls.length} 个工具调用
          </span>
        )}
        {hasTerminated && (
          <span className="text-xs text-red-600 bg-red-50 px-1.5 py-0.5 rounded flex items-center gap-1">
            <AlertTriangle className="w-3 h-3" />
            循环终止
          </span>
        )}
        {!hasToolCalls && !hasTerminated && step.thinking && (
          <span className="text-xs text-neutral-400">最终回答</span>
        )}
      </button>
      {expanded && (
        <div className="px-3 pb-3 pt-1 space-y-2 border-t border-neutral-100">
          {step.thinking && (
            <div className="text-sm text-neutral-700 whitespace-pre-wrap leading-relaxed">
              {step.thinking}
            </div>
          )}
          {step.toolCalls.map((tc) => (
            <ToolCallBlock key={tc.call_id || tc.tool} tc={tc} />
          ))}
          {hasTerminated && (
            <div className="text-xs text-red-600 bg-red-50 border border-red-200 rounded-md px-3 py-2">
              {step.loopTerminated!.reason === 'max_iterations' && '已达最大迭代次数上限'}
              {step.loopTerminated!.reason === 'time_budget' && '已超过整轮执行时间上限'}
              {step.loopTerminated!.reason === 'token_budget' && '已超过累计 token 上限'}
              {step.loopTerminated!.reason === 'stuck' && '检测到连续重复的工具调用'}
            </div>
          )}
        </div>
      )}
    </div>
  )
}
