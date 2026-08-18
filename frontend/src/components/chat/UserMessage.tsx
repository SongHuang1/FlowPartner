import type { Message } from '@/types'

export function UserMessage({ message }: { message: Message }) {
  return (
    <div className="flex justify-end">
      <div className="max-w-[75%]">
        <div className="rounded-lg px-4 py-2 text-sm bg-blue-500 text-white">
          {message.content}
        </div>
      </div>
    </div>
  )
}
