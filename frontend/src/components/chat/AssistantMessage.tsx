import { useState, useCallback } from 'react'
import Markdown from 'react-markdown'
import type { Components } from 'react-markdown'
import remarkGfm from 'remark-gfm'
import remarkMath from 'remark-math'
import katex from 'katex'
import { Loader2, CheckCircle2, XCircle } from 'lucide-react'
import 'katex/dist/katex.min.css'
import type { Message, ContentBlock } from '@/types'
import { MessageToolbar } from './MessageToolbar'

interface AssistantMessageProps {
  message: Message
  streamingContent?: string
}

export function AssistantMessage({ message }: AssistantMessageProps) {
  const isCompleted = message.status === 'completed'
  const blocks = message.content_blocks || []
  const copyContent = blocks.filter(b => b.type === 'text').map(b => b.content).join('')

  const [expandedAgent, setExpandedAgent] = useState<string | null>(null)

  const handleLinkClick = useCallback((e: React.MouseEvent<HTMLAnchorElement>) => {
    e.preventDefault()
    const url = e.currentTarget.href
    window.flowPartner.openExternal(url)
  }, [])

  const mdComponents: Components = {
    a: (props) => (
      <a
        {...props}
        onClick={handleLinkClick}
        target="_blank"
        rel="noopener noreferrer"
        className="text-blue-600 hover:underline"
      />
    ),
    pre: (props) => {
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
    code: ({ className, children, ...rest }) => {
      const hasLanguage = /language-(\w+)/.exec(className || '')
      if (!hasLanguage) {
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

  const renderSubagentBlock = (block: Extract<ContentBlock, { type: 'subagent' }>, idx: number) => {
    const isExpanded = expandedAgent === block.span_id
    const key = block.span_id || `subagent_${idx}`
    return (
      <span key={key} className="inline">
        <span className="inline-flex items-center gap-1 text-sm text-neutral-500">
          <span className="font-medium text-neutral-600">「{block.agent_name}」</span>
          {block.status === 'running' && <Loader2 className="w-3 h-3 animate-spin text-blue-500" />}
          {block.status !== 'running' && block.result && <span>：{block.result}</span>}
          {block.status === 'error' && <span>（执行失败）</span>}
          {(block.result || block.error) && (
            <button
              type="button"
              onClick={() => setExpandedAgent(isExpanded ? null : (block.span_id || String(idx)))}
              className="text-blue-500 hover:underline text-xs ml-1"
            >
              {isExpanded ? '收起' : '详情'}
            </button>
          )}
        </span>
        {isExpanded && (block.result || block.error) && (
          <div className="mt-1 mb-1 text-sm text-neutral-800 prose prose-sm max-w-none">
            <Markdown remarkPlugins={[remarkGfm, remarkMath]} components={mdComponents}>
              {block.error || block.result || ''}
            </Markdown>
          </div>
        )}
      </span>
    )
  }

  const renderToolCallBlock = (block: Extract<ContentBlock, { type: 'tool_call' }>, idx: number) => {
    const key = block.call_id || `tool_${idx}`
    let argsDisplay = ''
    try {
      argsDisplay = JSON.stringify(JSON.parse(block.arguments), null, 2)
    } catch {
      argsDisplay = block.arguments
    }
    return (
      <div key={key} className="rounded-lg border border-neutral-200 bg-neutral-50 overflow-hidden">
        <div className="flex items-center gap-2 px-3 py-2">
          {block.status === 'running' ? (
            <Loader2 className="w-3.5 h-3.5 text-blue-500 animate-spin shrink-0" />
          ) : block.status === 'done' ? (
            <CheckCircle2 className="w-3.5 h-3.5 text-green-500 shrink-0" />
          ) : (
            <XCircle className="w-3.5 h-3.5 text-red-500 shrink-0" />
          )}
          <span className="text-sm font-medium text-neutral-700">{block.tool_name}</span>
        </div>
        {argsDisplay && argsDisplay !== '{}' && (
          <div className="px-3 pb-1">
            <div className="text-xs text-neutral-500 font-mono bg-white rounded border border-neutral-100 p-2 max-h-24 overflow-y-auto whitespace-pre-wrap break-all">
              {argsDisplay}
            </div>
          </div>
        )}
        {(block.result || block.error) && (
          <div className="px-3 pb-2 pt-1">
            <div className="text-xs text-neutral-600 font-mono bg-white rounded border border-neutral-100 p-2 max-h-32 overflow-y-auto whitespace-pre-wrap break-all">
              {block.error || block.result}
            </div>
          </div>
        )}
      </div>
    )
  }

  const renderBlock = (block: ContentBlock, idx: number) => {
    switch (block.type) {
      case 'text':
        if (!block.content?.trim()) return null
        return (
          <div key={idx} className="text-sm text-neutral-800 prose prose-sm max-w-none">
            <Markdown remarkPlugins={[remarkGfm, remarkMath]} components={mdComponents}>
              {block.content}
            </Markdown>
          </div>
        )
      case 'tool_call':
        return renderToolCallBlock(block, idx)
      case 'subagent':
        return renderSubagentBlock(block, idx)
      default:
        return null
    }
  }

  return (
    <div className="flex justify-start">
      <div className="w-full min-w-0">
        <div className="text-xs text-neutral-500 mb-1">FlowPartner</div>
        <div className="space-y-2">
          {blocks.map((block, i) => renderBlock(block, i))}
        </div>
        {isCompleted && <MessageToolbar content={copyContent} />}
      </div>
    </div>
  )
}
