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
      <h2 className="text-lg font-medium text-neutral-700 mb-4">
        你好！我是 FlowPartner
      </h2>
      <div className="w-full max-w-2xl space-y-3">
        <ChatInput
          value={inputValue}
          onChange={onInputChange}
          onSend={onSend}
          disabled={disabled}
          loading={loading}
        />
        <div className="flex items-center justify-between px-1">
          {onExecutorChange && (
            <AgentSelector
              agents={agents}
              value={executorAgentId || ''}
              onChange={onExecutorChange}
              disabled={disabled || loading}
            />
          )}
          <div className="text-xs text-neutral-400 flex items-center gap-2">
            <span>模型: {settings.model}</span>
            <span className="text-neutral-300">|</span>
            <span>上下文: {settings.context_window}</span>
          </div>
        </div>
        {settings.working_directory && (
          <div className="text-xs text-neutral-400 text-center">
            工作路径: {settings.working_directory}
          </div>
        )}
      </div>
    </div>
  )
}
