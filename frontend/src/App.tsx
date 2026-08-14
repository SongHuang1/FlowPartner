import { useState } from 'react'
import { TitleBar } from '@/components/layout/TitleBar'
import { ActivityBar } from '@/components/layout/ActivityBar'
import { ChatArea } from '@/components/chat/ChatArea'
import { SettingsModal } from '@/components/settings/SettingsModal'

export default function App() {
  const [settingsOpen, setSettingsOpen] = useState(false)

  return (
    <div className="h-screen w-screen flex flex-col overflow-hidden font-sans">
      <TitleBar />
      <div className="flex flex-1 overflow-hidden">
        <ActivityBar onSettingsClick={() => setSettingsOpen(true)} />
        <ChatArea />
      </div>
      <SettingsModal open={settingsOpen} onClose={() => setSettingsOpen(false)} />
    </div>
  )
}
