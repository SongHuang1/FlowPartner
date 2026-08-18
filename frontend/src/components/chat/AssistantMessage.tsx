import { useCallback } from 'react'
import Markdown from 'react-markdown'
import remarkGfm from 'remark-gfm'
import type { Message } from '@/types'
import { MessageToolbar } from './MessageToolbar'

export function AssistantMessage({ message }: { message: Message }) {
  const isCompleted = message.status === 'completed'

  const handleLinkClick = useCallback((e: React.MouseEvent<HTMLAnchorElement>) => {
    e.preventDefault()
    const url = e.currentTarget.href
    window.flowPartner.openExternal(url)
  }, [])

  return (
    <div className="flex justify-start">
      <div className="w-full">
        <div className="text-xs text-neutral-500 mb-1">FlowPartner</div>
        <div className="text-sm text-neutral-800 prose prose-sm max-w-none">
          <Markdown
            remarkPlugins={[remarkGfm]}
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
                const { inline, className, children, ...rest } = props as {
                  inline?: boolean
                  className?: string
                  children?: React.ReactNode
                }
                if (inline) {
                  return (
                    <code className="bg-neutral-100 px-1 py-0.5 rounded text-sm font-mono">
                      {children}
                    </code>
                  )
                }
                const match = /language-(\w+)/.exec(className || '')
                const language = match?.[1]
                return (
                  <div className="relative">
                    {language && (
                      <div className="absolute top-0 left-0 px-2 py-0.5 text-xs text-neutral-500 bg-neutral-200 rounded-tl rounded-br font-mono">
                        {language}
                      </div>
                    )}
                    <pre className="bg-neutral-100 pt-6 p-3 rounded overflow-y-auto max-h-[400px]">
                      <code className="font-mono text-sm" {...rest}>
                        {children}
                      </code>
                    </pre>
                  </div>
                )
              },
            }}
          >
            {message.content}
          </Markdown>
        </div>
        {isCompleted && <MessageToolbar content={message.content} />}
      </div>
    </div>
  )
}
