import { X } from 'lucide-react'
import { cn } from '@/lib/utils'
import type { SidebarView } from './ActivityBar'
import { Button } from '@/components/ui/button'
import { APISettings } from '@/components/settings/APISettings'
import { AgentSettings } from '@/components/settings/AgentSettings'

interface SidebarProps {
  visible: boolean
  activeView: SidebarView
  onClose: () => void
}

function ConversationPanel() {
  return (
    <div className="flex flex-col gap-4 p-4">
      <h2 className="font-semibold text-base text-neutral-800">Welcome to FlowPartner</h2>
      <p className="text-sm text-neutral-600">Start a new chat or continue previous conversations</p>
      <div className="flex flex-col gap-2">
        <h3 className="text-xs font-medium text-neutral-500 uppercase tracking-wide">Suggested actions</h3>
        <Button variant="outline" className="justify-start text-sm" disabled>
          Start new chat
        </Button>
        <Button variant="outline" className="justify-start text-sm" disabled>
          View history
        </Button>
      </div>
    </div>
  )
}

function SettingsPanel() {
  return (
    <div className="flex flex-col gap-4 p-4">
      <h2 className="font-semibold text-base text-neutral-800">Settings</h2>
      <APISettings />
      <div className="border-t border-neutral-100" />
      <AgentSettings />
    </div>
  )
}

export function Sidebar({ visible, activeView, onClose }: SidebarProps) {
  return (
    <div
      data-testid="sidebar-panel"
      className={cn(
        "border-r border-neutral-200 bg-white flex flex-col shrink-0 overflow-hidden transition-all duration-200",
        visible ? "w-64" : "w-0"
      )}
      aria-hidden={!visible}
    >
      <div className="w-64 flex flex-col h-full">
        <div className="flex items-center justify-between p-3 border-b border-neutral-100">
          <span className="text-sm font-medium text-neutral-700">
            {activeView === 'conversation' ? 'Chat' : 'Settings'}
          </span>
          <Button variant="ghost" size="icon" className="w-7 h-7" onClick={onClose} aria-label="Collapse sidebar">
            <X className="w-4 h-4" />
          </Button>
        </div>
        <div className="flex-1 overflow-y-auto">
          {activeView === 'conversation' ? <ConversationPanel /> : <SettingsPanel />}
        </div>
      </div>
    </div>
  )
}
