import { useState } from 'react'
import { Eye, EyeOff, Lock, Unlock, KeyRound, Trash2 } from 'lucide-react'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { useSettings } from '@/hooks/useSettings'
import { useLock } from '@/hooks/useLock'
import { saveApiKey, clearApiKey } from '@/lib/api'
import { isPasswordStrong } from '@/lib/validation'

export function APISettings() {
  const { settings, updateSettings } = useSettings()
  const { lockStatus, unlock, lock } = useLock()
  const [showApiKey, setShowApiKey] = useState(false)
  const [apiKeyInput, setApiKeyInput] = useState('')
  const [password, setPassword] = useState('')
  const [passwordConfirm, setPasswordConfirm] = useState('')
  const [localError, setLocalError] = useState<string | null>(null)
  const [showPasswordDialog, setShowPasswordDialog] = useState(false)

  const handleUnlock = async () => {
    setLocalError(null)
    try {
      await unlock(password)
      setPassword('')
    } catch (e) {
      setLocalError(e instanceof Error ? e.message : 'Unlock failed')
    }
  }

  const handleLock = async () => {
    setLocalError(null)
    try {
      await lock()
    } catch (e) {
      setLocalError(e instanceof Error ? e.message : 'Lock failed')
    }
  }

  const handleShowPasswordDialog = () => {
    setLocalError(null)
    if (!apiKeyInput.trim()) {
      setLocalError('Please enter an API Key')
      return
    }
    setPassword('')
    setPasswordConfirm('')
    setShowPasswordDialog(true)
  }

  const handleConfirmSave = async () => {
    setLocalError(null)
    if (!isPasswordStrong(password)) {
      setLocalError('Password must be at least 8 characters with uppercase, lowercase and numbers')
      return
    }
    if (password !== passwordConfirm) {
      setLocalError('The two passwords do not match')
      return
    }
    try {
      await saveApiKey(apiKeyInput.trim(), password)
      setApiKeyInput('')
      setPassword('')
      setPasswordConfirm('')
      setShowPasswordDialog(false)
    } catch (e) {
      setLocalError(e instanceof Error ? e.message : 'Save failed')
    }
  }

  const handleClearApiKey = async () => {
    setLocalError(null)
    try {
      await clearApiKey()
      setApiKeyInput('')
      setPassword('')
      setPasswordConfirm('')
    } catch (e) {
      setLocalError(e instanceof Error ? e.message : 'Failed to clear API Key')
    }
  }

  return (
    <div className="flex flex-col gap-4">
      <h3 className="text-sm font-medium text-neutral-700">API Settings</h3>

      {localError && (
        <div className="text-sm text-red-500 bg-red-50 px-3 py-2 rounded-md">
          {localError}
        </div>
      )}

      <div className="flex flex-col gap-1">
        <label htmlFor="api-base-url" className="text-xs font-medium text-neutral-600">Base URL</label>
        <Input
          id="api-base-url"
          value={settings.base_url}
          onChange={(e) => updateSettings({ base_url: e.target.value })}
          placeholder="https://api.openai.com/v1"
        />
      </div>

      <div className="flex flex-col gap-1">
        <label htmlFor="api-model-name" className="text-xs font-medium text-neutral-600">Model name</label>
        <Input
          id="api-model-name"
          value={settings.model_name}
          onChange={(e) => updateSettings({ model_name: e.target.value })}
          placeholder="gpt-4"
        />
      </div>

      <div className="flex flex-col gap-1">
        <label htmlFor="api-key-input" className="text-xs font-medium text-neutral-600">API Key</label>
        <div className="flex gap-2">
          <div className="relative flex-1">
            <Input
              id="api-key-input"
              type={showApiKey ? 'text' : 'password'}
              value={apiKeyInput}
              onChange={(e) => setApiKeyInput(e.target.value)}
              placeholder={lockStatus.has_api_key ? 'Configured (enter a new value to change)' : 'Enter API Key'}
              disabled={!lockStatus.locked && lockStatus.has_api_key}
            />
            <button
              type="button"
              className="absolute right-2 top-1/2 -translate-y-1/2 text-neutral-400 hover:text-neutral-600"
              onClick={() => setShowApiKey(!showApiKey)}
              aria-label={showApiKey ? 'Hide' : 'Show'}
            >
              {showApiKey ? <EyeOff className="w-4 h-4" /> : <Eye className="w-4 h-4" />}
            </button>
          </div>
        </div>
      </div>

      <div className="flex gap-2 items-center">
        {lockStatus.locked ? (
          <div className="flex gap-2 items-center flex-1">
            <Input
              type="password"
              value={password}
              onChange={(e) => setPassword(e.target.value)}
              placeholder="Enter password to unlock"
              className="flex-1"
            />
            <Button onClick={handleUnlock} size="sm" className="flex items-center gap-1">
              <Unlock className="w-3 h-3" /> Unlock
            </Button>
          </div>
        ) : (
          <Button onClick={handleLock} size="sm" variant="outline" className="flex items-center gap-1">
            <Lock className="w-3 h-3" /> Lock
          </Button>
        )}
      </div>

      {lockStatus.has_api_key && (
        <div className="flex gap-2 items-center text-xs text-neutral-500">
          <KeyRound className="w-3 h-3" />
          <span>API Key configured</span>
        </div>
      )}

      <div className="flex gap-2">
        <Button
          onClick={handleShowPasswordDialog}
          size="sm"
          disabled={lockStatus.locked || !apiKeyInput.trim()}
          className="flex items-center gap-1"
        >
          <KeyRound className="w-3 h-3" /> Save API Key
        </Button>
        <Button
          onClick={handleClearApiKey}
          size="sm"
          variant="outline"
          disabled={!lockStatus.has_api_key}
          className="flex items-center gap-1"
        >
          <Trash2 className="w-3 h-3" /> Clear
        </Button>
      </div>

      {showPasswordDialog && (
        <div className="border border-neutral-200 rounded-md p-3 flex flex-col gap-3">
          <p className="text-xs text-neutral-600">Set protection password (min 8 chars, upper + lower + numbers)</p>
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
          <div className="flex gap-2">
            <Button onClick={handleConfirmSave} size="sm">Confirm</Button>
            <Button onClick={() => setShowPasswordDialog(false)} size="sm" variant="outline">Cancel</Button>
          </div>
        </div>
      )}
    </div>
  )
}
