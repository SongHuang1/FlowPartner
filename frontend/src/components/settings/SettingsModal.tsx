import { useState } from 'react'
import { X, Key, Bot, Settings } from 'lucide-react'
import { Button } from '@/components/ui/button'
import { APISettings } from './APISettings'
import { AgentSettings } from './AgentSettings'
import { CloseBehaviorSettings } from './CloseBehaviorSettings'

interface SettingsModalProps {
  open: boolean
  onClose: () => void
}

type TabId = 'api' | 'agent' | 'behavior'

const tabs: { id: TabId; label: string; icon: typeof Key }[] = [
  { id: 'api', label: 'API 配置', icon: Key },
  { id: 'agent', label: '智能体', icon: Bot },
  { id: 'behavior', label: '行为', icon: Settings },
]

export function SettingsModal({ open, onClose }: SettingsModalProps) {
  const [activeTab, setActiveTab] = useState<TabId>('api')

  if (!open) return null

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/40">
      <div className="bg-white rounded-xl shadow-2xl w-[560px] max-h-[85vh] flex flex-col overflow-hidden">
        <div className="flex items-center justify-between px-6 py-4 border-b border-neutral-200 bg-neutral-50">
          <h2 className="text-lg font-semibold text-neutral-800">设置</h2>
          <Button variant="ghost" size="icon" className="w-8 h-8 rounded-full" onClick={onClose} aria-label="关闭">
            <X className="w-4 h-4" />
          </Button>
        </div>

        <div className="flex border-b border-neutral-200">
          {tabs.map(({ id, label, icon: Icon }) => (
            <button
              key={id}
              onClick={() => setActiveTab(id)}
              className={
                'flex items-center gap-2 px-6 py-3 text-sm font-medium transition-colors relative ' +
                (activeTab === id
                  ? 'text-blue-600 bg-white'
                  : 'text-neutral-500 hover:text-neutral-700 hover:bg-neutral-50')
              }
            >
              <Icon className="w-4 h-4" />
              {label}
              {activeTab === id && (
                <span className="absolute bottom-0 left-0 right-0 h-0.5 bg-blue-600" />
              )}
            </button>
          ))}
        </div>

        <div className="flex-1 overflow-y-auto p-6">
          {activeTab === 'api' && <APISettings />}
          {activeTab === 'agent' && <AgentSettings />}
          {activeTab === 'behavior' && <CloseBehaviorSettings />}
        </div>
      </div>
    </div>
  )
}
