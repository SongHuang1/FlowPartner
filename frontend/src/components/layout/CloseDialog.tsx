import { useState, useEffect } from 'react'
import { X } from 'lucide-react'

interface CloseDialogProps {
  onMinimize: () => void
  onQuit: () => void
  onClose: () => void
  onRememberAction: (behavior: 'minimize' | 'quit') => void
}

export function CloseDialog({ onMinimize, onQuit, onClose, onRememberAction }: CloseDialogProps) {
  const [showQuitConfirm, setShowQuitConfirm] = useState(false)
  const [remember, setRemember] = useState(false)

  useEffect(() => {
    const handleEsc = (e: KeyboardEvent) => {
      if (e.key === 'Escape') onClose()
    }
    window.addEventListener('keydown', handleEsc)
    return () => window.removeEventListener('keydown', handleEsc)
  }, [onClose])

  const handleMinimize = () => {
    onMinimize()
    if (remember) {
      onRememberAction('minimize')
    }
  }

  const handleQuit = () => {
    if (!showQuitConfirm) {
      setShowQuitConfirm(true)
      return
    }
    onQuit()
    if (remember) {
      onRememberAction('quit')
    }
  }

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/40" onClick={onClose}>
      <div className="bg-white rounded-xl shadow-2xl w-80 flex flex-col overflow-hidden" onClick={e => e.stopPropagation()}>
        <div className="flex items-center justify-between px-5 py-4 border-b border-neutral-100">
          <h3 className="text-sm font-semibold text-neutral-800">
            {showQuitConfirm ? '确认退出' : '关闭窗口'}
          </h3>
          <button
            onClick={onClose}
            className="w-7 h-7 rounded-full flex items-center justify-center text-neutral-400 hover:text-neutral-600 hover:bg-neutral-100 transition-colors"
            aria-label="关闭"
          >
            <X className="w-4 h-4" />
          </button>
        </div>

        <div className="p-5">
          {showQuitConfirm ? (
            <div className="flex flex-col gap-4">
              <p className="text-sm text-neutral-600">确定要退出 FlowPartner 吗？</p>
              <div className="flex gap-2 justify-end">
                <button
                  onClick={handleQuit}
                  className="px-4 py-2 text-sm font-medium text-white bg-red-500 rounded-lg hover:bg-red-600 transition-colors"
                >
                  确认退出
                </button>
                <button
                  onClick={() => setShowQuitConfirm(false)}
                  className="px-4 py-2 text-sm font-medium text-neutral-600 bg-neutral-100 rounded-lg hover:bg-neutral-200 transition-colors"
                >
                  取消
                </button>
              </div>
            </div>
          ) : (
            <div className="flex flex-col gap-3">
              <p className="text-sm text-neutral-600">点击关闭按钮后希望发生什么？</p>
              <div className="flex flex-col gap-2">
                <button
                  onClick={handleMinimize}
                  className="w-full px-4 py-3 text-sm font-medium text-neutral-700 bg-neutral-50 border border-neutral-200 rounded-lg hover:bg-neutral-100 hover:border-neutral-300 transition-colors"
                >
                  最小化到托盘
                </button>
                <button
                  onClick={handleQuit}
                  className="w-full px-4 py-3 text-sm font-medium text-red-600 bg-red-50 border border-red-100 rounded-lg hover:bg-red-100 hover:border-red-200 transition-colors"
                >
                  完全退出
                </button>
              </div>
              <label className="flex items-center gap-2 text-xs text-neutral-500 cursor-pointer pt-1">
                <input
                  type="checkbox"
                  checked={remember}
                  onChange={(e) => setRemember(e.target.checked)}
                  className="rounded border-neutral-300"
                />
                记住本次选择
              </label>
            </div>
          )}
        </div>
      </div>
    </div>
  )
}
