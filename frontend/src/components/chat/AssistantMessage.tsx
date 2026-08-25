import { useState, useCallback } from 'react'
import Markdown from 'react-markdown'
import remarkGfm from 'remark-gfm'
import remarkMath from 'remark-math'
import katex from 'katex'
import { Loader2, ChevronRight } from 'lucide-react'
import 'katex/dist/katex.min.css'
import type { Message, ContentBlock } from '@/types'
import { MessageToolbar } from './MessageToolbar'

interface AssistantMessageProps {
  message: Message
  streamingContent?: string
}

export function AssistantMessage({ message, streamingContent }: AssistantMessageProps) {
  const isCompleted = message.status === 'completed'
  const isStreaming = message.status === 'streaming'
  const contentBlocks = message.content_blocks
  const hasBlocks = contentBlocks && contentBlocks.length > 0
  const displayContent = !hasBlocks && isStreaming && streamingContent ? streamingContent : message.content
  const [expandedAgent, setExpandedAgent] = useState<string | null>(null)

  const handleLinkClick = useCallback((e: React.MouseEvent<HTMLAnchorElement>) => {
    e.preventDefault()
    const url = e.currentTarget.href
    window.flowPartner.openExternal(url)
  }, [])

  // eslint-disable-next-line @typescript-eslint/no-explicit-any
  const mdComponents: any = {
    a: (props: any) => (
      <a
        {...props}
        onClick={handleLinkClick}
        target="_blank"
        rel="noopener noreferrer"
        className="text-blue-600 hover:underline"
      />
    ),
    pre: (props: any) => {
      const { children } = props
      const child = children as { props?: { className?: string; children?: React.ReactNode } } | undefined
      if (child?.props?.className?.includes('math-display')) {
        const text = String(child.props.children || '')
        try {
          const html = katex.renderToString(text, { displayMode: true, throwOnError: false })
          return <div className="katex-block" dangerouslySetInnerHTML={{ __html: html }} />
        } catch {
          return <pre {...props} />
        }
      }
      return <pre {...props} />
    },
    code: (props: any) => {
      const { inline, className, children, ...rest } = props
      const hasLanguage = /language-(\w+)/.exec(className || '')
      if (inline || !hasLanguage) {
        return <code className="bg-neutral-100 px-1 py-0.5 rounded text-pink-600 text-[0.875em] font-mono" {...rest}>{children}</code>
      }
      return (
        <div className="relative my-3">
          {className && (
            <div className="absolute top-0 left-0 px-3 py-1 text-xs text-neutral-500 bg-neutral-200 rounded-tl rounded-br font-mono z-10">
              {className.replace('language-', '')}
            </div>
          )}
          <pre className="bg-neutral-900 text-neutral-100 pt-8 p-4 rounded-lg overflow-x-auto max-h-[400px]">
            <code className="font-mono text-sm leading-relaxed" {...rest}>{children}</code>
          </pre>
        </div>
      )
    },
  }

  console.log('[AssistantMessage] render:', { hasBlocks, blockCount: contentBlocks?.length, blocks: contentBlocks?.map(b => ({ type: b.type, len: b.type === 'text' ? b.content?.length : undefined })) })

  const renderSubagentBlock = (block: Extract<ContentBlock, { type: 'subagent' }>, idx: number) => {
    const isExpanded = expandedAgent === block.span_id
    const key = block.span_id || `subagent_${idx}`
    return (
      <div key={key} className="rounded-lg border border-neutral-200 bg-neutral-50 overflow-hidden">
        <button
          type="button"
          onClick={() => setExpandedAgent(isExpanded ? null : (block.span_id || String(idx)))}
          className="w-full flex items-center gap-2 px-3 py-2 text-left hover:bg-neutral-100 transition-colors"
        >
          <ChevronRight className={`w-3.5 h-3.5 text-neutral-400 transition-transform shrink-0 ${isExpanded ? 'rotate-90' : ''}`} />
          <span className="text-sm font-medium text-neutral-700">{block.agent_name}</span>
          {block.task && <span className="text-xs text-neutral-400 truncate flex-1">{block.task}</span>}
          {block.status === 'running' && (
            <span className="text-xs text-blue-600 flex items-center gap-1 shrink-0">
              <Loader2 className="w-3 h-3 animate-spin" />
              执行中
            </span>
          )}
          {block.status === 'done' && <span className="text-xs text-green-600 shrink-0">已完成</span>}
          {block.status === 'error' && <span className="text-xs text-red-500 shrink-0">失败</span>}
        </button>
        {isExpanded && (block.result || block.error) && (
          <div className="px-3 py-2 text-sm text-neutral-800 prose prose-sm max-w-none border-t border-neutral-200">
            <Markdown remarkPlugins={[remarkGfm, remarkMath]} components={mdComponents}>
              {block.error || block.result || ''}
            </Markdown>
          </div>
        )}
      </div>
    )
  }

  return (
    <div className="flex justify-start">
      <div className="w-full min-w-0">
        <div className="text-xs text-neutral-500 mb-1">FlowPartner</div>
        {hasBlocks ? (
          <div className="space-y-2">
            {contentBlocks.map((block, i) =>
              block.type === 'text' ? (
                block.content.trim() ? (
                  <div key={i} className="text-sm text-neutral-800 prose prose-sm max-w-none">
                    <Markdown remarkPlugins={[remarkGfm, remarkMath]} components={mdComponents}>
                      {block.content}
                    </Markdown>
                  </div>
                ) : null
              ) : (
                renderSubagentBlock(block, i)
              )
            )}
          </div>
        ) : (
          <div className="text-sm text-neutral-800 prose prose-sm max-w-none">
            <Markdown remarkPlugins={[remarkGfm, remarkMath]} components={mdComponents}>
              {displayContent}
            </Markdown>
          </div>
        )}
        {isCompleted && <MessageToolbar content={message.content} />}
      </div>
    </div>
  )
}
