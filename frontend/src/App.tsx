import { useState } from 'react'
import { TitleBar } from '@/components/layout/TitleBar'
import { ActivityBar, type SidebarView } from '@/components/layout/ActivityBar'
import { Sidebar } from '@/components/layout/Sidebar'
import { StatusBar } from '@/components/layout/StatusBar'
import { ChatArea } from '@/components/chat/ChatArea'

export default function App() {
  const [activeSidebarView, setActiveSidebarView] = useState<SidebarView>('conversation')
  const [sidebarVisible, setSidebarVisible] = useState(true)

  const handleActivitySelect = (view: SidebarView) => {
    if (view === activeSidebarView && sidebarVisible) {
      setSidebarVisible(false)
    } else {
      setActiveSidebarView(view)
      setSidebarVisible(true)
    }
  }

  return (
    <div className="h-screen w-screen flex flex-col overflow-hidden font-sans">
      <TitleBar />
      <div className="flex flex-1 overflow-hidden">
        <ActivityBar activeView={activeSidebarView} onSelect={handleActivitySelect} />
        <Sidebar visible={sidebarVisible} activeView={activeSidebarView} onClose={() => setSidebarVisible(false)} />
        <ChatArea />
      </div>
      <StatusBar />
    </div>
  )
}
