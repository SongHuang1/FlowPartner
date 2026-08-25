import type { Message } from '@/types'

function renderUserContent(content: string, agentNames: Set<string>) {
  const parts = content.split(/(@[^\s]+)/g)
  return parts.map((part, i) => {
    if (part.length > 1 && part.startsWith('@') && agentNames.has(part.slice(1))) {
      return (
        <span key={i} className="font-bold bg-white/25 rounded px-1 py-0.5" data-testid="agent-mention">
          {part}
        </span>
      )
    }
    return <span key={i}>{part}</span>
  })
}

interface UserMessageProps {
  message: Message
  agentNames?: Set<string>
}

export function UserMessage({ message, agentNames = new Set<string>() }: UserMessageProps) {
  return (
    <div className="flex justify-end">
      <div className="max-w-[75%]">
        <div className="rounded-lg px-4 py-2 text-sm bg-blue-500 text-white whitespace-pre-wrap break-words">
          {renderUserContent(message.content, agentNames)}
        </div>
      </div>
    </div>
  )
}
