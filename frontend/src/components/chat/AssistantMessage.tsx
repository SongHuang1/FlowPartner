import { useCallback } from 'react'
import Markdown from 'react-markdown'
import remarkGfm from 'remark-gfm'
import remarkMath from 'remark-math'
import rehypeKatex from 'rehype-katex'
import 'katex/dist/katex.min.css'
import type { Message } from '@/types'
import { MessageToolbar } from './MessageToolbar'

interface AssistantMessageProps {
  message: Message
  streamingContent?: string
}

export function AssistantMessage({ message, streamingContent }: AssistantMessageProps) {
  const isCompleted = message.status === 'completed'
  const isStreaming = message.status === 'streaming'
  const content = isStreaming && streamingContent ? streamingContent : message.content

  const handleLinkClick = useCallback((e: React.MouseEvent<HTMLAnchorElement>) => {
    e.preventDefault()
    const url = e.currentTarget.href
    window.flowPartner.openExternal(url)
  }, [])

  return (
    <div className="flex justify-start">
      <div className="w-full min-w-0">
        <div className="text-xs text-neutral-500 mb-1">FlowPartner</div>
        <div className="text-sm text-neutral-800 prose prose-sm max-w-none">
          <Markdown
            remarkPlugins={[remarkGfm, remarkMath]}
            rehypePlugins={[rehypeKatex]}
            components={{
              a: (props) => (
                <a
                  {...props}
                  onClick={handleLinkClick}
                  target="_blank"
                  rel="noopener noreferrer"
                  className="text-blue-600 hover:underline"
                />
              ),
              code: (props) => {
                const { inline, className, children, ...rest } = props as { inline?: boolean; className?: string; children?: React.ReactNode; [key: string]: unknown }
                const hasLanguage = /language-(\w+)/.exec(className || '')
                if (inline || !hasLanguage) {
                  return (
                    <code className="bg-neutral-100 px-1 py-0.5 rounded text-pink-600 text-[0.875em] font-mono" {...rest}>
                      {children}
                    </code>
                  )
                }
                return (
                  <div className="relative my-3">
                    {className && (
                      <div className="absolute top-0 left-0 px-3 py-1 text-xs text-neutral-500 bg-neutral-200 rounded-tl rounded-br font-mono z-10">
                        {className.replace('language-', '')}
                      </div>
                    )}
                    <pre className="bg-neutral-900 text-neutral-100 pt-8 p-4 rounded-lg overflow-x-auto max-h-[400px]">
                      <code className="font-mono text-sm leading-relaxed" {...rest}>
                        {children}
                      </code>
                    </pre>
                  </div>
                )
              },
            }}
          >
            {content}
          </Markdown>
        </div>
        {isCompleted && <MessageToolbar content={message.content} />}
      </div>
    </div>
  )
}
