import { Bot } from 'lucide-react'
import type { AgentMeta } from '@/types'

interface AgentSelectorProps {
  agents: AgentMeta[]
  value: string
  onChange: (agentId: string) => void
  disabled?: boolean
}

export function AgentSelector({ agents, value, onChange, disabled }: AgentSelectorProps) {
  return (
    <div className="flex items-center gap-2">
      <Bot className="w-4 h-4 text-neutral-400" />
      <select
        value={value || 'main'}
        onChange={(e) => onChange(e.target.value)}
        disabled={disabled || agents.length === 0}
        aria-label="会话执行者"
        className="rounded-lg border border-neutral-200 bg-white px-3 py-1.5 text-sm text-neutral-700 shadow-sm focus:outline-none focus:ring-2 focus:ring-blue-500/20 focus:border-blue-400 disabled:opacity-50"
      >
        {agents.map((agent) => (
          <option key={agent.id} value={agent.id}>
            {agent.name}
          </option>
        ))}
      </select>
    </div>
  )
}