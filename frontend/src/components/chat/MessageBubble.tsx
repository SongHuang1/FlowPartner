import type { Message } from '@/types'

export function MessageBubble({ message }: { message: Message }) {
  const isUser = message.role === 'user'
  return (
    <div className={`flex ${isUser ? 'justify-end' : 'justify-start'}`}>
      <div className="max-w-[75%]">
        {!isUser && (
          <div className="text-xs text-neutral-500 mb-1">FlowPartner</div>
        )}
        <div className={`rounded-lg px-4 py-2 text-sm ${
          isUser
            ? 'bg-blue-500 text-white'
            : 'bg-neutral-100 text-neutral-800'
        }`}>
          {message.content}
        </div>
      </div>
    </div>
  )
}
