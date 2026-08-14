import { useState } from 'react'
import { X } from 'lucide-react'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { isPasswordStrong } from '@/lib/validation'

interface PasswordDialogProps {
  open: boolean
  onClose: () => void
  onConfirm: (password: string) => void
  title?: string
  description?: string
}

export function PasswordDialog({ open, onClose, onConfirm, title = 'Set protection password', description }: PasswordDialogProps) {
  const [password, setPassword] = useState('')
  const [passwordConfirm, setPasswordConfirm] = useState('')
  const [localError, setLocalError] = useState<string | null>(null)

  if (!open) return null

  const handleConfirm = () => {
    setLocalError(null)
    if (!isPasswordStrong(password)) {
      setLocalError('Password must be at least 8 characters with uppercase, lowercase and numbers')
      return
    }
    if (password !== passwordConfirm) {
      setLocalError('The two passwords do not match')
      return
    }
    onConfirm(password)
    setPassword('')
    setPasswordConfirm('')
    setLocalError(null)
  }

  const handleClose = () => {
    setPassword('')
    setPasswordConfirm('')
    setLocalError(null)
    onClose()
  }

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/50">
      <div className="bg-white rounded-lg shadow-lg w-80 p-4 flex flex-col gap-3">
        <div className="flex items-center justify-between">
          <h3 className="text-sm font-medium text-neutral-800">{title}</h3>
          <Button variant="ghost" size="icon" className="w-7 h-7" onClick={handleClose} aria-label="Close">
            <X className="w-4 h-4" />
          </Button>
        </div>

        {description && (
          <p className="text-xs text-neutral-600">{description}</p>
        )}

        {localError && (
          <div className="text-xs text-red-500 bg-red-50 px-3 py-2 rounded-md">
            {localError}
          </div>
        )}

        <Input
          type="password"
          value={password}
          onChange={(e) => setPassword(e.target.value)}
          placeholder="Enter password"
        />
        <Input
          type="password"
          value={passwordConfirm}
          onChange={(e) => setPasswordConfirm(e.target.value)}
          placeholder="Confirm password"
        />

        {!isPasswordStrong(password) && password.length > 0 && (
          <p className="text-xs text-amber-600">Password must be at least 8 characters with uppercase, lowercase and numbers</p>
        )}
        {password && password !== passwordConfirm && passwordConfirm.length > 0 && (
          <p className="text-xs text-red-500">The two passwords do not match</p>
        )}

        <div className="flex gap-2 justify-end">
          <Button onClick={handleConfirm} size="sm">Confirm</Button>
          <Button onClick={handleClose} size="sm" variant="outline">Cancel</Button>
        </div>
      </div>
    </div>
  )
}
