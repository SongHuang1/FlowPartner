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
      setLocalError(e instanceof Error ? e.message : '解锁失败')
    }
  }

  const handleLock = async () => {
    setLocalError(null)
    try {
      await lock()
    } catch (e) {
      setLocalError(e instanceof Error ? e.message : '上锁失败')
    }
  }

  const handleShowPasswordDialog = () => {
    setLocalError(null)
    if (!apiKeyInput.trim()) {
      setLocalError('请输入 API Key')
      return
    }
    setPassword('')
    setPasswordConfirm('')
    setShowPasswordDialog(true)
  }

  const handleConfirmSave = async () => {
    setLocalError(null)
    if (!isPasswordStrong(password)) {
      setLocalError('密码需≥8位，包含大小写字母和数字')
      return
    }
    if (password !== passwordConfirm) {
      setLocalError('两次输入的密码不一致')
      return
    }
    try {
      await saveApiKey(apiKeyInput.trim(), password)
      setApiKeyInput('')
      setPassword('')
      setPasswordConfirm('')
      setShowPasswordDialog(false)
    } catch (e) {
      setLocalError(e instanceof Error ? e.message : '保存失败')
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
      setLocalError(e instanceof Error ? e.message : '清除 API Key 失败')
    }
  }

  return (
    <div className="flex flex-col gap-4">
      <h3 className="text-sm font-medium text-neutral-700">API 配置</h3>

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
        <label htmlFor="api-model-name" className="text-xs font-medium text-neutral-600">模型名称</label>
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
              placeholder={lockStatus.has_api_key ? '已配置（输入新值以修改）' : '输入 API Key'}
              disabled={!lockStatus.locked && lockStatus.has_api_key}
            />
            <button
              type="button"
              className="absolute right-2 top-1/2 -translate-y-1/2 text-neutral-400 hover:text-neutral-600"
              onClick={() => setShowApiKey(!showApiKey)}
              aria-label={showApiKey ? '隐藏' : '显示'}
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
              placeholder="输入密码解锁"
              className="flex-1"
            />
            <Button onClick={handleUnlock} size="sm" className="flex items-center gap-1">
              <Unlock className="w-3 h-3" /> 解锁
            </Button>
          </div>
        ) : (
          <Button onClick={handleLock} size="sm" variant="outline" className="flex items-center gap-1">
            <Lock className="w-3 h-3" /> 上锁
          </Button>
        )}
      </div>

      {lockStatus.has_api_key && (
        <div className="flex gap-2 items-center text-xs text-neutral-500">
          <KeyRound className="w-3 h-3" />
          <span>API Key 已配置</span>
        </div>
      )}

      <div className="flex gap-2">
        <Button
          onClick={handleShowPasswordDialog}
          size="sm"
          disabled={lockStatus.locked || !apiKeyInput.trim()}
          className="flex items-center gap-1"
        >
          <KeyRound className="w-3 h-3" /> 保存 API Key
        </Button>
        <Button
          onClick={handleClearApiKey}
          size="sm"
          variant="outline"
          disabled={!lockStatus.has_api_key}
          className="flex items-center gap-1"
        >
          <Trash2 className="w-3 h-3" /> 清除
        </Button>
      </div>

      {showPasswordDialog && (
        <div className="border border-neutral-200 rounded-md p-3 flex flex-col gap-3">
          <p className="text-xs text-neutral-600">设置保护密码（≥8位，含大小写+数字）</p>
          <Input
            type="password"
            value={password}
            onChange={(e) => setPassword(e.target.value)}
            placeholder="输入密码"
          />
          <Input
            type="password"
            value={passwordConfirm}
            onChange={(e) => setPasswordConfirm(e.target.value)}
            placeholder="确认密码"
          />
          {!isPasswordStrong(password) && password.length > 0 && (
            <p className="text-xs text-amber-600">密码需≥8位，包含大小写字母和数字</p>
          )}
          {password && password !== passwordConfirm && passwordConfirm.length > 0 && (
            <p className="text-xs text-red-500">两次输入的密码不一致</p>
          )}
          <div className="flex gap-2">
            <Button onClick={handleConfirmSave} size="sm">确认</Button>
            <Button onClick={() => setShowPasswordDialog(false)} size="sm" variant="outline">取消</Button>
          </div>
        </div>
      )}
    </div>
  )
}
