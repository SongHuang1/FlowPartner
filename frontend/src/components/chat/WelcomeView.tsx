import type { Settings } from '@/types'
import { ChatInput } from './ChatArea'

interface WelcomeViewProps {
  settings: Settings
  inputValue: string
  onInputChange: (v: string) => void
  onSend: () => void
}

export function WelcomeView({ settings, inputValue, onInputChange, onSend }: WelcomeViewProps) {
  return (
    <div className="flex-1 flex flex-col items-center justify-center p-4">
      <h2 className="text-lg font-medium text-neutral-700 mb-4">
        你好！我是 FlowPartner
      </h2>
      <div className="w-full max-w-2xl">
        <ChatInput
          value={inputValue}
          onChange={onInputChange}
          onSend={onSend}
        />
      </div>
      <div className="text-xs text-neutral-400 text-center mt-4">
        <span>model: {settings.model}</span>
        <span className="mx-2">|</span>
        <span>agent: {settings.agent_id}</span>
        <span className="mx-2">|</span>
        <span>ctx: {settings.context_window}</span>
      </div>
      {settings.working_directory && (
        <div className="text-xs text-neutral-400 text-center mt-1">
          path: {settings.working_directory}
        </div>
      )}
    </div>
  )
}
