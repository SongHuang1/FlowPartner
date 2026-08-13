import { useState } from 'react'
import { ChevronDown, ChevronRight } from 'lucide-react'

interface EventDetailProps {
  eventType: string
  payload: string
}

function parsePayload(payload: string): Record<string, unknown> | null {
  try {
    return JSON.parse(payload) as Record<string, unknown>
  } catch {
    return null
  }
}

export function EventDetail({ eventType, payload }: EventDetailProps) {
  const [expanded, setExpanded] = useState(false)
  const parsed = parsePayload(payload)

  if (eventType === 'tool_call') {
    const tool = (parsed?.tool as string) || 'unknown'
    const args = (parsed?.args as Record<string, unknown>) || {}

    return (
      <div className="border border-neutral-200 rounded-md text-sm">
        <button
          onClick={() => setExpanded(!expanded)}
          className="w-full flex items-center gap-2 px-3 py-2 text-left hover:bg-neutral-50"
        >
          {expanded ? (
            <ChevronDown className="w-4 h-4 text-neutral-400" />
          ) : (
            <ChevronRight className="w-4 h-4 text-neutral-400" />
          )}
          <span className="font-mono text-blue-600">{tool}</span>
          <span className="text-neutral-400 text-xs">tool_call</span>
        </button>
        {expanded && (
          <div className="px-3 pb-2 pt-1 border-t border-neutral-100">
            <pre className="text-xs text-neutral-600 overflow-x-auto whitespace-pre-wrap">
              {JSON.stringify(args, null, 2)}
            </pre>
          </div>
        )}
      </div>
    )
  }

  if (eventType === 'tool_result') {
    const tool = (parsed?.tool as string) || 'unknown'
    const result = (parsed?.result as string) || ''

    return (
      <div className="border border-neutral-200 rounded-md text-sm">
        <button
          onClick={() => setExpanded(!expanded)}
          className="w-full flex items-center gap-2 px-3 py-2 text-left hover:bg-neutral-50"
        >
          {expanded ? (
            <ChevronDown className="w-4 h-4 text-neutral-400" />
          ) : (
            <ChevronRight className="w-4 h-4 text-neutral-400" />
          )}
          <span className="font-mono text-green-600">{tool}</span>
          <span className="text-neutral-400 text-xs">tool_result</span>
        </button>
        {expanded && (
          <div className="px-3 pb-2 pt-1 border-t border-neutral-100">
            <pre className="text-xs text-neutral-600 overflow-x-auto whitespace-pre-wrap">
              {result}
            </pre>
          </div>
        )}
      </div>
    )
  }

  return null
}
