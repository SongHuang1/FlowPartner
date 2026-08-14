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

  const handleUnlock = async () => {
    setLocalError(null)
    if (!password.trim()) {
      setLocalError('请输入密码')
      return
    }
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
      setLocalError(e instanceof Error ? e.message : '锁定失败')
    }
  }

  const handleSaveNewKey = async () => {
    setLocalError(null)
    if (!apiKeyInput.trim()) {
      setLocalError('请输入 API Key')
      return
    }
    if (!isPasswordStrong(password)) {
      setLocalError('密码至少 8 位，且需包含大写字母、小写字母和数字')
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

  const renderError = () => {
    if (!localError) return null
    return (
      <div className="text-sm text-red-500 bg-red-50 px-3 py-2 rounded-md">
        {localError}
      </div>
    )
  }

  const renderCommonFields = () => (
    <>
      <div className="flex flex-col gap-1">
        <label htmlFor="api-base-url" className="text-xs font-medium text-neutral-600">接口地址</label>
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
          value={settings.model}
          onChange={(e) => updateSettings({ model: e.target.value })}
          placeholder="gpt-4"
        />
      </div>
    </>
  )

  // Mode A: No API key configured — create new key + set password
  if (!lockStatus.has_api_key) {
    return (
      <div className="flex flex-col gap-4">
        <h3 className="text-sm font-medium text-neutral-700">API 设置</h3>
        {renderError()}
        {renderCommonFields()}
        <div className="border border-blue-200 bg-blue-50 rounded-md p-3 flex flex-col gap-3">
          <p className="text-xs text-blue-700 font-medium">新建 API Key</p>
          <div className="flex flex-col gap-1">
            <label htmlFor="api-key-new" className="text-xs font-medium text-neutral-600">API Key</label>
            <div className="relative">
              <Input
                id="api-key-new"
                type={showApiKey ? 'text' : 'password'}
                value={apiKeyInput}
                onChange={(e) => setApiKeyInput(e.target.value)}
                placeholder="输入 API Key"
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
          <div className="flex flex-col gap-1">
            <label htmlFor="api-password-new" className="text-xs font-medium text-neutral-600">设置保护密码</label>
            <Input
              id="api-password-new"
              type="password"
              value={password}
              onChange={(e) => setPassword(e.target.value)}
              placeholder="至少 8 位，含大写、小写字母和数字"
            />
            {!isPasswordStrong(password) && password.length > 0 && (
              <p className="text-xs text-amber-600">密码至少 8 位，且需包含大写字母、小写字母和数字</p>
            )}
          </div>
          <div className="flex flex-col gap-1">
            <label htmlFor="api-password-confirm" className="text-xs font-medium text-neutral-600">确认密码</label>
            <Input
              id="api-password-confirm"
              type="password"
              value={passwordConfirm}
              onChange={(e) => setPasswordConfirm(e.target.value)}
              placeholder="再次输入密码"
            />
            {password && password !== passwordConfirm && passwordConfirm.length > 0 && (
              <p className="text-xs text-red-500">两次输入的密码不一致</p>
            )}
          </div>
          <Button
            onClick={handleSaveNewKey}
            size="sm"
            disabled={!apiKeyInput.trim() || !password || !passwordConfirm}
            className="flex items-center gap-1 self-start"
          >
            <KeyRound className="w-3 h-3" /> 保存并加密
          </Button>
        </div>
      </div>
    )
  }

  // Mode B: API key configured, locked — unlock with password
  if (lockStatus.locked) {
    return (
      <div className="flex flex-col gap-4">
        <h3 className="text-sm font-medium text-neutral-700">API 设置</h3>
        {renderError()}
        {renderCommonFields()}
        <div className="border border-amber-200 bg-amber-50 rounded-md p-3 flex flex-col gap-3">
          <div className="flex items-center gap-2 text-xs text-amber-700">
            <Lock className="w-3 h-3" />
            <span className="font-medium">API Key 已加密，需解锁后才能使用</span>
          </div>
          <div className="flex gap-2 items-center">
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
        </div>
        <div className="flex gap-2 items-center text-xs text-neutral-500">
          <KeyRound className="w-3 h-3" />
          <span>API Key 已配置</span>
        </div>
      </div>
    )
  }

  // Mode C: Unlocked — full access
  return (
    <div className="flex flex-col gap-4">
      <h3 className="text-sm font-medium text-neutral-700">API 设置</h3>
      {renderError()}
      {renderCommonFields()}
      <div className="flex flex-col gap-1">
        <label htmlFor="api-key-input" className="text-xs font-medium text-neutral-600">API Key</label>
        <div className="flex gap-2">
          <div className="relative flex-1">
            <Input
              id="api-key-input"
              type={showApiKey ? 'text' : 'password'}
              value={apiKeyInput}
              onChange={(e) => setApiKeyInput(e.target.value)}
              placeholder="输入新 API Key 以修改"
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
      <div className="flex gap-2 flex-wrap">
        <Button
          onClick={handleSaveNewKey}
          size="sm"
          disabled={!apiKeyInput.trim() || !password.trim()}
          className="flex items-center gap-1"
        >
          <KeyRound className="w-3 h-3" /> 修改并重新加密
        </Button>
        <Button
          onClick={handleClearApiKey}
          size="sm"
          variant="outline"
          className="flex items-center gap-1"
        >
          <Trash2 className="w-3 h-3" /> 清除
        </Button>
        <Button onClick={handleLock} size="sm" variant="outline" className="flex items-center gap-1">
          <Lock className="w-3 h-3" /> 锁定
        </Button>
      </div>
    </div>
  )
}
