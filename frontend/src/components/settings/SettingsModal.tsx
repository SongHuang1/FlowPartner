import { X } from 'lucide-react'
import { Button } from '@/components/ui/button'
import { APISettings } from './APISettings'
import { AgentSettings } from './AgentSettings'
import { CloseBehaviorSettings } from './CloseBehaviorSettings'

interface SettingsModalProps {
  open: boolean
  onClose: () => void
}

export function SettingsModal({ open, onClose }: SettingsModalProps) {
  if (!open) return null

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/50">
      <div className="bg-white rounded-lg shadow-lg w-[480px] max-h-[80vh] flex flex-col">
        <div className="flex items-center justify-between p-4 border-b border-neutral-200">
          <h2 className="text-base font-semibold text-neutral-800">设置</h2>
          <Button variant="ghost" size="icon" className="w-8 h-8" onClick={onClose} aria-label="关闭">
            <X className="w-4 h-4" />
          </Button>
        </div>
        <div className="flex-1 overflow-y-auto p-4 flex flex-col gap-4">
          <APISettings />
          <div className="border-t border-neutral-100" />
          <AgentSettings />
          <div className="border-t border-neutral-100" />
          <CloseBehaviorSettings />
        </div>
      </div>
    </div>
  )
}
