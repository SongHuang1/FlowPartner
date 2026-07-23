import { useState } from 'react'
import { X } from 'lucide-react'
import { Button } from '@/components/ui/button'

interface CloseDialogProps {
  open: boolean
  onClose: () => void
  onMinimize: () => void
  onQuit: () => void
  onRemember: (remember: boolean, behavior: 'minimize' | 'quit') => void
}

export function CloseDialog({ open, onClose, onMinimize, onQuit, onRemember }: CloseDialogProps) {
  const [remember, setRemember] = useState(false)
  const [showQuitConfirm, setShowQuitConfirm] = useState(false)

  if (!open) return null

  const handleMinimize = () => {
    if (remember) {
      onRemember(true, 'minimize')
    } else {
      onMinimize()
    }
    onClose()
  }

  const handleQuit = () => {
    if (!showQuitConfirm) {
      setShowQuitConfirm(true)
      return
    }
    if (remember) {
      onRemember(true, 'quit')
    } else {
      onQuit()
    }
    onClose()
  }

  const handleClose = () => {
    setRemember(false)
    setShowQuitConfirm(false)
    onClose()
  }

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/50">
      <div className="bg-white rounded-lg shadow-lg w-80 p-4 flex flex-col gap-3">
        <div className="flex items-center justify-between">
          <h3 className="text-sm font-medium text-neutral-800">
            {showQuitConfirm ? '确认退出' : '关闭窗口'}
          </h3>
          <Button variant="ghost" size="icon" className="w-7 h-7" onClick={handleClose} aria-label="关闭">
            <X className="w-4 h-4" />
          </Button>
        </div>

        {showQuitConfirm ? (
          <p className="text-sm text-neutral-600">确定要退出 FlowPartner 吗？</p>
        ) : (
          <>
            <p className="text-sm text-neutral-600">点击关闭按钮时希望执行什么操作？</p>
            <div className="flex flex-col gap-2">
              <Button onClick={handleMinimize} variant="outline" className="justify-start text-sm">
                最小化到托盘
              </Button>
              <Button onClick={handleQuit} variant="outline" className="justify-start text-sm">
                完全退出
              </Button>
            </div>
            <label className="flex items-center gap-2 text-xs text-neutral-600 cursor-pointer">
              <input
                type="checkbox"
                checked={remember}
                onChange={(e) => setRemember(e.target.checked)}
                className="rounded border-neutral-300"
              />
              记住我的选择
            </label>
          </>
        )}

        {showQuitConfirm && (
          <div className="flex gap-2 justify-end">
            <Button onClick={handleQuit} size="sm">确认退出</Button>
            <Button onClick={() => setShowQuitConfirm(false)} size="sm" variant="outline">取消</Button>
          </div>
        )}
      </div>
    </div>
  )
}
