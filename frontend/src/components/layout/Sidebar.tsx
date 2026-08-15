import { X } from 'lucide-react'
import { cn } from '@/lib/utils'
import { Button } from '@/components/ui/button'

interface SidebarProps {
  visible: boolean
  onClose: () => void
}

export function Sidebar({ visible, onClose }: SidebarProps) {
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
          <span className="text-sm font-medium text-neutral-700">聊天记录</span>
          <Button variant="ghost" size="icon" className="w-7 h-7" onClick={onClose} aria-label="收起侧栏">
            <X className="w-4 h-4" />
          </Button>
        </div>
        <div className="flex-1 overflow-y-auto p-4">
          <div className="flex flex-col gap-4">
            <h2 className="font-semibold text-base text-neutral-800">欢迎使用 FlowPartner</h2>
            <p className="text-sm text-neutral-600">开始新对话或继续之前的对话</p>
            <div className="flex flex-col gap-2">
              <h3 className="text-xs font-medium text-neutral-500 uppercase tracking-wide">建议操作</h3>
              <Button variant="outline" className="justify-start text-sm" disabled>
                开始新对话
              </Button>
              <Button variant="outline" className="justify-start text-sm" disabled>
                查看历史
              </Button>
            </div>
          </div>
        </div>
      </div>
    </div>
  )
}
