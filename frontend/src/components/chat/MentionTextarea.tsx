import { useState, useRef, useEffect, useCallback, useMemo } from 'react'
import { Bot } from 'lucide-react'
import type { AgentMeta } from '@/types'
import { detectMention, type MentionState } from '@/lib/mention'

const MAX_MATCHES = 6

function sameMention(a: MentionState | null, b: MentionState | null): boolean {
  if (a === b) return true
  if (a === null || b === null) return false
  return a.start === b.start && a.query === b.query
}

interface MentionTextareaProps {
  value: string
  onChange: (v: string) => void
  onKeyDown?: (e: React.KeyboardEvent<HTMLTextAreaElement>) => void
  agents: AgentMeta[]
  disabled?: boolean
  placeholder?: string
  maxLength?: number
  rows?: number
  inputRef?: React.RefObject<HTMLTextAreaElement | null>
}

export function MentionTextarea({
  value,
  onChange,
  onKeyDown,
  agents,
  disabled,
  placeholder,
  maxLength,
  rows,
  inputRef,
}: MentionTextareaProps) {
  const [mention, setMention] = useState<MentionState | null>(null)
  const [highlightIndex, setHighlightIndex] = useState(0)
  const innerRef = useRef<HTMLTextAreaElement | null>(null)
  const overlayRef = useRef<HTMLDivElement | null>(null)
  const pendingCursorRef = useRef<number | null>(null)
  const composingRef = useRef(false)

  const setRefs = (el: HTMLTextAreaElement | null) => {
    innerRef.current = el
    if (inputRef) inputRef.current = el
  }

  const agentNames = useMemo(() => new Set(agents.map((a) => a.name)), [agents])

  const matches = useMemo(() => {
    if (!mention) return []
    const q = mention.query.toLowerCase()
    return agents.filter((a) => a.name.toLowerCase().includes(q)).slice(0, MAX_MATCHES)
  }, [mention, agents])

  const open = !disabled && mention !== null && matches.length > 0

  // 选中补全后恢复光标到插入文本末尾（等待受控 value 提交到 DOM）
  useEffect(() => {
    if (pendingCursorRef.current !== null && innerRef.current) {
      const pos = pendingCursorRef.current
      innerRef.current.setSelectionRange(pos, pos)
      pendingCursorRef.current = null
    }
  }, [value])

  const updateMention = useCallback(() => {
    if (composingRef.current) return
    const el = innerRef.current
    if (!el) return
    const next = detectMention(el.value, el.selectionStart ?? el.value.length)
    setMention((prev) => (sameMention(prev, next) ? prev : next))
    setHighlightIndex(0)
  }, [])

  const applyMention = useCallback(
    (agent: AgentMeta) => {
      const el = innerRef.current
      if (!mention || !el) return
      const insert = `@${agent.name} `
      const cursor = el.selectionStart ?? el.value.length
      const before = value.slice(0, mention.start)
      const after = value.slice(cursor)
      pendingCursorRef.current = before.length + insert.length
      onChange(before + insert + after)
      setMention(null)
    },
    [mention, value, onChange],
  )

  const handleChange = (e: React.ChangeEvent<HTMLTextAreaElement>) => {
    onChange(e.target.value)
    updateMention()
  }

  const handleKeyDown = (e: React.KeyboardEvent<HTMLTextAreaElement>) => {
    if (open) {
      if (e.key === 'ArrowDown') {
        e.preventDefault()
        setHighlightIndex((i) => (i + 1) % matches.length)
        return
      }
      if (e.key === 'ArrowUp') {
        e.preventDefault()
        setHighlightIndex((i) => (i - 1 + matches.length) % matches.length)
        return
      }
      if (e.key === 'Enter' || e.key === 'Tab') {
        e.preventDefault()
        applyMention(matches[highlightIndex])
        return
      }
      if (e.key === 'Escape') {
        e.preventDefault()
        setMention(null)
        return
      }
    }
    onKeyDown?.(e)
  }

  const handleScroll = (e: React.UIEvent<HTMLTextAreaElement>) => {
    if (overlayRef.current) {
      overlayRef.current.scrollTop = e.currentTarget.scrollTop
    }
  }

  const parts = value.split(/(@[^\s]+)/g)

  return (
    <div className="relative flex-1 min-w-0">
      <div
        ref={overlayRef}
        aria-hidden
        className="pointer-events-none absolute inset-0 overflow-hidden whitespace-pre-wrap break-words rounded-md border border-transparent px-3 py-2 text-sm"
      >
        {parts.map((part, i) =>
          part.startsWith('@') && part.length > 1 && agentNames.has(part.slice(1)) ? (
            <span key={i} className="font-semibold text-blue-600">
              {part}
            </span>
          ) : (
            <span key={i}>{part}</span>
          ),
        )}
      </div>
      <textarea
        ref={setRefs}
        value={value}
        onChange={handleChange}
        onKeyDown={handleKeyDown}
        onKeyUp={updateMention}
        onClick={updateMention}
        onScroll={handleScroll}
        onCompositionStart={() => {
          composingRef.current = true
        }}
        onCompositionEnd={() => {
          composingRef.current = false
          updateMention()
        }}
        onBlur={() => {
          // 延迟关闭，让弹层项的 mousedown 先完成选中
          setTimeout(() => setMention(null), 150)
        }}
        placeholder={placeholder}
        maxLength={maxLength}
        rows={rows}
        disabled={disabled}
        data-testid="mention-textarea"
        className="relative w-full resize-none overflow-y-auto rounded-md border border-neutral-200 bg-transparent px-3 py-2 text-sm caret-neutral-800 placeholder:text-neutral-500 focus:outline-none focus:ring-1 focus:ring-neutral-300 disabled:opacity-50 min-h-[40px] max-h-[200px]"
      />
      {open && (
        <ul
          data-testid="mention-popup"
          className="absolute bottom-full left-0 z-10 mb-1 w-full max-w-sm overflow-hidden rounded-md border border-neutral-200 bg-white shadow-lg"
          role="listbox"
        >
          {matches.map((agent, i) => (
            <li key={agent.id}>
              <button
                type="button"
                role="option"
                aria-selected={i === highlightIndex}
                onMouseDown={(e) => {
                  e.preventDefault()
                  applyMention(agent)
                }}
                onMouseEnter={() => setHighlightIndex(i)}
                className={`flex w-full items-center gap-2 px-3 py-2 text-left text-sm transition-colors ${
                  i === highlightIndex ? 'bg-neutral-100' : 'bg-white hover:bg-neutral-50'
                }`}
              >
                <Bot className="h-4 w-4 shrink-0 text-blue-600" />
                <span className="shrink-0 font-medium text-neutral-800">{agent.name}</span>
                {agent.description && (
                  <span className="truncate text-xs text-neutral-400">{agent.description}</span>
                )}
              </button>
            </li>
          ))}
        </ul>
      )}
    </div>
  )
}
