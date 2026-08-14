import { useState } from 'react'
import { Eye, EyeOff, Lock, Unlock, KeyRound, Trash2, Plus } from 'lucide-react'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { useSettings } from '@/hooks/useSettings'
import { useLock } from '@/hooks/useLock'
import { saveApiKey, clearApiKey } from '@/lib/api'
import { isPasswordStrong } from '@/lib/validation'

interface ModelConfig {
  id: string
  name: string
  base_url: string
  model_name: string
  encrypted_api_key?: string
  temperature: number
  response_format: string
  timeout_secs: number
}

export function APISettings() {
  const { settings, updateSettings } = useSettings()
  const { lockStatus, unlock, lock } = useLock()
  const [showApiKey, setShowApiKey] = useState(false)
  const [apiKeyInput, setApiKeyInput] = useState('')
  const [password, setPassword] = useState('')
  const [passwordConfirm, setPasswordConfirm] = useState('')
  const [localError, setLocalError] = useState<string | null>(null)
  const [showAddConfig, setShowAddConfig] = useState(false)
  const [newConfigName, setNewConfigName] = useState('')
  const [newConfigBaseUrl, setNewConfigBaseUrl] = useState('')
  const [newConfigModel, setNewConfigModel] = useState('')
  const [newConfigKey, setNewConfigKey] = useState('')
  const [newConfigPassword, setNewConfigPassword] = useState('')

  const modelConfigs: ModelConfig[] = (settings as unknown as { model_configs: ModelConfig[] }).model_configs || []

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
      await saveApiKey(apiKeyInput.trim(), password, settings.model, settings.base_url)
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

  const handleAddConfig = async () => {
    setLocalError(null)
    if (!newConfigName.trim()) {
      setLocalError('请输入配置名称')
      return
    }
    if (!newConfigBaseUrl.trim()) {
      setLocalError('请输入接口地址')
      return
    }
    if (!newConfigModel.trim()) {
      setLocalError('请输入模型名称')
      return
    }
    if (!newConfigKey.trim()) {
      setLocalError('请输入 API Key')
      return
    }
    if (!isPasswordStrong(newConfigPassword)) {
      setLocalError('密码至少 8 位，且需包含大写字母、小写字母和数字')
      return
    }

    const newConfig: ModelConfig = {
      id: `config-${Date.now()}`,
      name: newConfigName.trim(),
      base_url: newConfigBaseUrl.trim(),
      model_name: newConfigModel.trim(),
      temperature: 0.7,
      response_format: 'text',
      timeout_secs: 30,
    }

    const updatedConfigs = [...modelConfigs, newConfig]
    try {
      await saveApiKey(newConfigKey.trim(), newConfigPassword, newConfigModel.trim(), newConfigBaseUrl.trim())
      updateSettings({ model_configs: updatedConfigs } as Partial<typeof settings> & { model_configs: ModelConfig[] })
      setNewConfigName('')
      setNewConfigBaseUrl('')
      setNewConfigModel('')
      setNewConfigKey('')
      setNewConfigPassword('')
      setShowAddConfig(false)
    } catch (e) {
      setLocalError(e instanceof Error ? e.message : '新增配置失败')
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

      {modelConfigs.length > 0 && (
        <div className="flex flex-col gap-2">
          <label className="text-xs font-medium text-neutral-600">已保存的配置</label>
          {modelConfigs.map((cfg) => (
            <div key={cfg.id} className="flex items-center justify-between p-2 border border-neutral-200 rounded-md text-xs">
              <div className="flex flex-col">
                <span className="font-medium text-neutral-700">{cfg.name}</span>
                <span className="text-neutral-500">{cfg.base_url} · {cfg.model_name}</span>
              </div>
              <Button variant="ghost" size="sm" className="text-red-500 hover:text-red-600">
                <Trash2 className="w-3 h-3" />
              </Button>
            </div>
          ))}
        </div>
      )}

      {showAddConfig ? (
        <div className="border border-blue-200 bg-blue-50 rounded-md p-3 flex flex-col gap-3">
          <p className="text-xs text-blue-700 font-medium">新增配置</p>
          <div className="flex flex-col gap-1">
            <label htmlFor="new-config-name" className="text-xs font-medium text-neutral-600">配置名称</label>
            <Input
              id="new-config-name"
              value={newConfigName}
              onChange={(e) => setNewConfigName(e.target.value)}
              placeholder="如：OpenAI 备用"
            />
          </div>
          <div className="flex flex-col gap-1">
            <label htmlFor="new-config-url" className="text-xs font-medium text-neutral-600">接口地址</label>
            <Input
              id="new-config-url"
              value={newConfigBaseUrl}
              onChange={(e) => setNewConfigBaseUrl(e.target.value)}
              placeholder="https://api.openai.com/v1"
            />
          </div>
          <div className="flex flex-col gap-1">
            <label htmlFor="new-config-model" className="text-xs font-medium text-neutral-600">模型名称</label>
            <Input
              id="new-config-model"
              value={newConfigModel}
              onChange={(e) => setNewConfigModel(e.target.value)}
              placeholder="gpt-4"
            />
          </div>
          <div className="flex flex-col gap-1">
            <label htmlFor="new-config-key" className="text-xs font-medium text-neutral-600">API Key</label>
            <Input
              id="new-config-key"
              type="password"
              value={newConfigKey}
              onChange={(e) => setNewConfigKey(e.target.value)}
              placeholder="输入 API Key"
            />
          </div>
          <div className="flex flex-col gap-1">
            <label htmlFor="new-config-password" className="text-xs font-medium text-neutral-600">保护密码</label>
            <Input
              id="new-config-password"
              type="password"
              value={newConfigPassword}
              onChange={(e) => setNewConfigPassword(e.target.value)}
              placeholder="至少 8 位，含大写、小写字母和数字"
            />
            {!isPasswordStrong(newConfigPassword) && newConfigPassword.length > 0 && (
              <p className="text-xs text-amber-600">密码至少 8 位，且需包含大写字母、小写字母和数字</p>
            )}
          </div>
          <div className="flex gap-2">
            <Button
              onClick={handleAddConfig}
              size="sm"
              disabled={!newConfigName.trim() || !newConfigBaseUrl.trim() || !newConfigModel.trim() || !newConfigKey.trim() || !isPasswordStrong(newConfigPassword)}
              className="flex items-center gap-1"
            >
              <Plus className="w-3 h-3" /> 保存配置
            </Button>
            <Button onClick={() => setShowAddConfig(false)} size="sm" variant="outline">
              取消
            </Button>
          </div>
        </div>
      ) : (
        <Button
          onClick={() => setShowAddConfig(true)}
          size="sm"
          variant="outline"
          className="flex items-center gap-1 self-start"
        >
          <Plus className="w-3 h-3" /> 新增配置
        </Button>
      )}

      <div className="flex flex-col gap-1">
        <label htmlFor="api-key-input" className="text-xs font-medium text-neutral-600">
          API Key <span className="text-neutral-400 font-normal">（修改当前密钥）</span>
        </label>
        <div className="relative">
          <Input
            id="api-key-input"
            type={showApiKey ? 'text' : 'password'}
            value={apiKeyInput}
            onChange={(e) => setApiKeyInput(e.target.value)}
            placeholder="输入新 API Key 以替换"
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
        <label htmlFor="api-password-input" className="text-xs font-medium text-neutral-600">
          保护密码 <span className="text-neutral-400 font-normal">（修改 API Key 时需要）</span>
        </label>
        <Input
          id="api-password-input"
          type="password"
          value={password}
          onChange={(e) => setPassword(e.target.value)}
          placeholder="输入保护密码"
        />
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
