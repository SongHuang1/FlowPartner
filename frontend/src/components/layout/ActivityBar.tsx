import { MessageSquare, Settings } from 'lucide-react'
import { Button } from '@/components/ui/button'
import { Tooltip } from '@/components/ui/tooltip'
import { cn } from '@/lib/utils'

export type SidebarView = 'conversation' | 'settings'

interface ActivityBarProps {
  activeView: SidebarView
  onSelect: (view: SidebarView) => void
}

export function ActivityBar({ activeView, onSelect }: ActivityBarProps) {
  const items: { view: SidebarView; icon: typeof MessageSquare; label: string }[] = [
    { view: 'conversation', icon: MessageSquare, label: '对话' },
    { view: 'settings', icon: Settings, label: '设置' },
  ]

  return (
    <div className="w-12 flex flex-col items-center py-2 border-r border-neutral-200 bg-neutral-100 gap-1 shrink-0">
      {items.map(({ view, icon: Icon, label }) => (
        <Tooltip key={view} content={label}>
          <Button
            variant="outline"
            size="icon"
            className={cn(
              'w-9 h-9 border-neutral-200 bg-white shadow-sm',
              activeView === view && 'bg-blue-500 text-white border-blue-500 hover:bg-blue-600 hover:border-blue-600'
            )}
            onClick={() => onSelect(view)}
            aria-label={label}
          >
            <Icon className="w-5 h-5" />
          </Button>
        </Tooltip>
      ))}
    </div>
  )
}
