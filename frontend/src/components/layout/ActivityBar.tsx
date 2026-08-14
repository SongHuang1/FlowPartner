import { MessageSquare, Settings } from 'lucide-react'
import { Button } from '@/components/ui/button'
import { Tooltip } from '@/components/ui/tooltip'
import { cn } from '@/lib/utils'

interface ActivityBarProps {
  onSettingsClick: () => void
}

export function ActivityBar({ onSettingsClick }: ActivityBarProps) {
  return (
    <div className="w-12 flex flex-col items-center py-2 border-r border-neutral-200 bg-neutral-100 gap-1 shrink-0">
      <Tooltip content="聊天">
        <Button
          variant="outline"
          size="icon"
          className={cn(
            'w-9 h-9 border-neutral-200 bg-white shadow-sm',
            'bg-blue-500 text-white border-blue-500 hover:bg-blue-600 hover:border-blue-600'
          )}
          aria-label="聊天"
        >
          <MessageSquare className="w-5 h-5" />
        </Button>
      </Tooltip>
      <div className="flex-1" />
      <Tooltip content="设置">
        <Button
          variant="outline"
          size="icon"
          className="w-9 h-9 border-neutral-200 bg-white shadow-sm"
          onClick={onSettingsClick}
          aria-label="设置"
        >
          <Settings className="w-5 h-5" />
        </Button>
      </Tooltip>
    </div>
  )
}
