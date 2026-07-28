import { useState } from 'react'
import { X, Lock } from 'lucide-react'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'

interface UnlockDialogProps {
  open: boolean
  onClose: () => void
  onUnlock: (password: string) => Promise<void>
  failedAttempts: number
  lockedUntil?: string
}

export function UnlockDialog({ open, onClose, onUnlock, failedAttempts, lockedUntil }: UnlockDialogProps) {
  const [password, setPassword] = useState('')
  const [localError, setLocalError] = useState<string | null>(null)
  const [loading, setLoading] = useState(false)

  if (!open) return null

  const isLocked = lockedUntil && new Date(lockedUntil) > new Date()

  const handleUnlock = async () => {
    if (!password) {
      setLocalError('请输入密码')
      return
    }
    setLoading(true)
    setLocalError(null)
    try {
      await onUnlock(password)
      setPassword('')
      onClose()
    } catch (e) {
      setLocalError(e instanceof Error ? e.message : '解锁失败')
      setPassword('')
    } finally {
      setLoading(false)
    }
  }

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/50">
      <div className="bg-white rounded-lg shadow-lg w-80 p-4 flex flex-col gap-3">
        <div className="flex items-center justify-between">
          <div className="flex items-center gap-2">
            <Lock className="w-4 h-4 text-neutral-600" />
            <h3 className="text-sm font-medium text-neutral-800">解锁 API Key</h3>
          </div>
          <Button variant="ghost" size="icon" className="w-7 h-7" onClick={onClose} aria-label="关闭">
            <X className="w-4 h-4" />
          </Button>
        </div>

        {isLocked && (
          <div className="text-xs text-red-500 bg-red-50 px-3 py-2 rounded-md">
            账户已锁定，请 {lockedUntil ? new Date(lockedUntil).toLocaleTimeString() : '稍后'} 再试
          </div>
        )}

        {localError && (
          <div className="text-xs text-red-500 bg-red-50 px-3 py-2 rounded-md">
            {localError}
          </div>
        )}

        {failedAttempts > 0 && !isLocked && (
          <p className="text-xs text-amber-600">
            已失败 {failedAttempts} 次，连续 5 次错误将锁定 30 秒
          </p>
        )}

        <Input
          type="password"
          value={password}
          onChange={(e) => setPassword(e.target.value)}
          placeholder="输入保护密码"
          disabled={isLocked || loading}
        />

        <div className="flex gap-2 justify-end">
          <Button onClick={handleUnlock} size="sm" disabled={isLocked || loading}>
            {loading ? '解锁中...' : '解锁'}
          </Button>
          <Button onClick={onClose} size="sm" variant="outline">取消</Button>
        </div>
      </div>
    </div>
  )
}
