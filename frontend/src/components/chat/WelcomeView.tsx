import { useEffect, useState } from 'react'
import type { Settings, AgentMeta } from '@/types'
import { ChatInput } from './ChatArea'
import { AgentSelector } from './AgentSelector'
import { listAgents } from '@/lib/api'

interface WelcomeViewProps {
  settings: Settings
  inputValue: string
  onInputChange: (v: string) => void
  onSend: () => void
  disabled?: boolean
  loading?: boolean
  executorAgentId?: string
  onExecutorChange?: (agentId: string) => void
}

export function WelcomeView({ settings, inputValue, onInputChange, onSend, disabled, loading, executorAgentId, onExecutorChange }: WelcomeViewProps) {
  const [agents, setAgents] = useState<AgentMeta[]>([])

  useEffect(() => {
    let cancelled = false
    listAgents()
      .then((items) => {
        if (!cancelled) setAgents(items)
      })
      .catch(() => {})
    return () => { cancelled = true }
  }, [])

  return (
    <div className="flex-1 flex flex-col items-center justify-center p-4">
      <h2 className="text-lg font-medium text-neutral-700 mb-6">
        你好！我是 FlowPartner
      </h2>
      <div className="w-full max-w-2xl">
        <ChatInput
          value={inputValue}
          onChange={onInputChange}
          onSend={onSend}
          disabled={disabled}
          loading={loading}
        />
        <div className="mt-4 flex flex-col gap-2 px-1">
          {onExecutorChange && (
            <div className="flex items-center justify-between">
              <AgentSelector
                agents={agents}
                value={executorAgentId || ''}
                onChange={onExecutorChange}
                disabled={disabled || loading}
              />
              <span className="text-xs text-neutral-400">
                输入 @智能体名 可指定子智能体
              </span>
            </div>
          )}
          <div className="flex items-center justify-between text-xs text-neutral-400">
            <span>模型: {settings.model} · 上下文: {settings.context_window}</span>
            {settings.working_directory && <span>路径: {settings.working_directory}</span>}
          </div>
        </div>
      </div>
    </div>
  )
}
