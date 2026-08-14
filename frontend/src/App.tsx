import { useState, useEffect } from 'react'
import { TitleBar } from '@/components/layout/TitleBar'
import { ActivityBar } from '@/components/layout/ActivityBar'
import { ChatArea } from '@/components/chat/ChatArea'
import { SettingsModal } from '@/components/settings/SettingsModal'
import { CloseDialog } from '@/components/layout/CloseDialog'

export default function App() {
  const [settingsOpen, setSettingsOpen] = useState(false)
  const [closeDialogOpen, setCloseDialogOpen] = useState(false)

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
  }

  return (
    <div className="h-screen w-screen flex flex-col overflow-hidden font-sans">
      <TitleBar />
      <div className="flex flex-1 overflow-hidden">
        <ActivityBar onSettingsClick={() => setSettingsOpen(true)} />
        <ChatArea />
      </div>
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
