import { useState, useEffect } from 'react'
import { TitleBar } from '@/components/layout/TitleBar'
import { ActivityBar } from '@/components/layout/ActivityBar'
import { Sidebar } from '@/components/layout/Sidebar'
import { StatusBar } from '@/components/layout/StatusBar'
import { ChatArea } from '@/components/chat/ChatArea'
import { SettingsModal } from '@/components/settings/SettingsModal'
import { CloseDialog } from '@/components/layout/CloseDialog'
import { useSettings } from '@/hooks/useSettings'
import { useConversation } from '@/hooks/useConversation'

export default function App() {
  const [settingsOpen, setSettingsOpen] = useState(false)
  const [closeDialogOpen, setCloseDialogOpen] = useState(false)
  const [sidebarVisible, setSidebarVisible] = useState(true)
  const { updateSettings } = useSettings()
  const conversation = useConversation()

  useEffect(() => {
    window.flowPartner.onCloseAction(() => {
      setCloseDialogOpen(true)
    })
  }, [])

  const handleCloseAction = (action: 'minimize' | 'quit') => {
    setCloseDialogOpen(false)
    window.flowPartner.sendCloseAction(action)
  }

  const handleRememberAction = (behavior: 'minimize' | 'quit') => {
    window.flowPartner.updateCloseBehavior(behavior, true)
    updateSettings({ close_behavior: behavior, close_remembered: true })
  }

  return (
    <div className="h-screen w-screen flex flex-col overflow-hidden font-sans">
      <TitleBar />
      <div className="flex flex-1 overflow-hidden">
        <ActivityBar
          onChatClick={() => setSidebarVisible(!sidebarVisible)}
          onSettingsClick={() => setSettingsOpen(true)}
        />
        <Sidebar
          visible={sidebarVisible}
          onClose={() => setSidebarVisible(false)}
          onNewChat={conversation.startNewConversation}
          onLoadSession={conversation.loadConversation}
        />
        <ChatArea conversation={conversation} />
      </div>
      <StatusBar />
      <SettingsModal open={settingsOpen} onClose={() => setSettingsOpen(false)} />
      {closeDialogOpen && (
        <CloseDialog
          onMinimize={() => handleCloseAction('minimize')}
          onQuit={() => handleCloseAction('quit')}
          onClose={() => setCloseDialogOpen(false)}
          onRememberAction={handleRememberAction}
        />
      )}
    </div>
  )
}
