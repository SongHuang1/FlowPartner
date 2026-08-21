import { useState, useEffect } from 'react'
import { Eye, EyeOff, Lock, Unlock, KeyRound, Trash2, Plus, Check } from 'lucide-react'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { useSettings } from '@/hooks/useSettings'
import { useLock } from '@/hooks/useLock'
import { clearApiKey, saveSettings } from '@/lib/api'
import type { Settings } from '@/types'
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
  const { settings, updateSettings, getCurrentSettings, refreshSettings } = useSettings()
  const { lockStatus, unlock, lock, refreshStatus } = useLock()
  const [showApiKey, setShowApiKey] = useState(false)
  const [apiKeyInput, setApiKeyInput] = useState('')
  const [password, setPassword] = useState('')
  const [passwordConfirm, setPasswordConfirm] = useState('')
  const [localError, setLocalError] = useState<string | null>(null)
  const [localSuccess, setLocalSuccess] = useState<string | null>(null)
  const [showAddConfig, setShowAddConfig] = useState(false)
  const [newConfigName, setNewConfigName] = useState('')
  const [newConfigBaseUrl, setNewConfigBaseUrl] = useState('')
  const [newConfigModel, setNewConfigModel] = useState('')
  const [newConfigKey, setNewConfigKey] = useState('')
  const [newConfigPassword, setNewConfigPassword] = useState('')
  const [newConfigPasswordConfirm, setNewConfigPasswordConfirm] = useState('')
  const [deletingId, setDeletingId] = useState<string | null>(null)
  const [firstConfigName, setFirstConfigName] = useState('')

  const modelConfigs: ModelConfig[] = (settings as unknown as { model_configs: ModelConfig[] }).model_configs || []
  const activeConfigId = (settings as unknown as { active_config_id: string }).active_config_id || []

  useEffect(() => {
    if (localSuccess) {
      const timer = setTimeout(() => setLocalSuccess(null), 3000)
      return () => clearTimeout(timer)
    }
  }, [localSuccess])

  const handleUnlock = async () => {
    setLocalError(null)
    setLocalSuccess(null)
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
    setLocalSuccess(null)
    try {
      await lock()
    } catch (e) {
      setLocalError(e instanceof Error ? e.message : '锁定失败')
    }
  }

  const handleSaveCurrentKey = async () => {
    setLocalError(null)
    setLocalSuccess(null)
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
      const current = getCurrentSettings()
      const fullSettings = {
        ...current,
        api_key: apiKeyInput.trim(),
        password,
      }
      await saveSettings(fullSettings as Settings & { api_key: string; password: string })
      await refreshSettings()
      setApiKeyInput('')
      setPassword('')
      setPasswordConfirm('')
      await refreshStatus()
      setLocalSuccess('API Key 已更新')
    } catch (e) {
      setLocalError(e instanceof Error ? e.message : '保存失败')
    }
  }

  const handleClearApiKey = async () => {
    setLocalError(null)
    setLocalSuccess(null)
    try {
      await clearApiKey()
      await refreshSettings()
      setApiKeyInput('')
      setPassword('')
      setPasswordConfirm('')
      await refreshStatus()
      setLocalSuccess('API Key 已清除')
    } catch (e) {
      setLocalError(e instanceof Error ? e.message : '清除 API Key 失败')
    }
  }

  const handleAddConfig = async () => {
    setLocalError(null)
    setLocalSuccess(null)
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
    if (newConfigPassword !== newConfigPasswordConfirm) {
      setLocalError('两次输入的密码不一致')
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
      const current = getCurrentSettings()
      const fullSettings = {
        ...current,
        model: newConfigModel.trim(),
        base_url: newConfigBaseUrl.trim(),
        api_key: newConfigKey.trim(),
        password: newConfigPassword,
        model_configs: updatedConfigs,
        active_config_id: newConfig.id,
      }
      await saveSettings(fullSettings as Settings & { api_key: string; password: string })
      await refreshSettings()
      setNewConfigName('')
      setNewConfigBaseUrl('')
      setNewConfigModel('')
      setNewConfigKey('')
      setNewConfigPassword('')
      setNewConfigPasswordConfirm('')
      setShowAddConfig(false)
      await refreshStatus()
      setLocalSuccess('新配置已添加')
    } catch (e) {
      setLocalError(e instanceof Error ? e.message : '新增配置失败')
    }
  }

  const handleDeleteConfig = async (configId: string) => {
    setLocalError(null)
    setLocalSuccess(null)
    const updatedConfigs = modelConfigs.filter(c => c.id !== configId)
    const newActiveId = activeConfigId === configId
      ? (updatedConfigs[0]?.id || '')
      : activeConfigId

    try {
      updateSettings({ model_configs: updatedConfigs, active_config_id: newActiveId } as typeof settings & { model_configs: ModelConfig[]; active_config_id: string })
      setDeletingId(null)
      await refreshStatus()
      setLocalSuccess('配置已删除')
    } catch (e) {
      setLocalError(e instanceof Error ? e.message : '删除失败')
    }
  }

  const handleSetActive = (configId: string) => {
    updateSettings({ active_config_id: configId } as Partial<typeof settings>)
  }

  const handleFirstTimeSave = async () => {
    setLocalError(null)
    setLocalSuccess(null)
    if (!firstConfigName.trim()) {
      setLocalError('请输入配置名称')
      return
    }
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

    const current = getCurrentSettings()

    const newConfig: ModelConfig = {
      id: `config-${Date.now()}`,
      name: firstConfigName.trim(),
      base_url: current.base_url,
      model_name: current.model,
      temperature: 0.7,
      response_format: 'text',
      timeout_secs: 30,
    }

    try {
      const fullSettings = {
        ...current,
        model: current.model,
        base_url: current.base_url,
        api_key: apiKeyInput.trim(),
        password,
        model_configs: [newConfig],
        active_config_id: newConfig.id,
      }
      await saveSettings(fullSettings as Settings & { api_key: string; password: string })
      await refreshSettings()
      setApiKeyInput('')
      setPassword('')
      setPasswordConfirm('')
      setFirstConfigName('')
      await refreshStatus()
      setLocalSuccess('API Key 配置成功')
    } catch (e) {
      setLocalError(e instanceof Error ? e.message : '保存失败')
    }
  }



  const renderMessage = () => {
    if (localError) {
      return (
        <div className="text-sm text-red-600 bg-red-50 px-4 py-3 rounded-lg border border-red-100">
          {localError}
        </div>
      )
    }
    if (localSuccess) {
      return (
        <div className="text-sm text-green-600 bg-green-50 px-4 py-3 rounded-lg border border-green-100">
          {localSuccess}
        </div>
      )
    }
    return null
  }

  // Mode A: No API key configured
  if (!lockStatus.has_api_key) {
    return (
      <div className="flex flex-col gap-4">
        {renderMessage()}
        <div className="border border-blue-200 bg-blue-50/50 rounded-lg p-4 flex flex-col gap-4">
          <h4 className="text-sm font-medium text-blue-800">新建 API 配置</h4>
          <div className="grid grid-cols-2 gap-3">
            <div className="flex flex-col gap-1.5">
              <label htmlFor="cfg-name" className="text-xs font-medium text-neutral-600">配置名称</label>
              <Input id="cfg-name" value={firstConfigName} onChange={(e) => setFirstConfigName(e.target.value)} placeholder="如：OpenAI 主账号" />
            </div>
            <div className="flex flex-col gap-1.5">
              <label htmlFor="cfg-model" className="text-xs font-medium text-neutral-600">模型名称</label>
              <Input id="cfg-model" value={settings.model} onChange={(e) => updateSettings({ model: e.target.value })} placeholder="gpt-4" />
            </div>
          </div>
          <div className="flex flex-col gap-1.5">
            <label htmlFor="cfg-url" className="text-xs font-medium text-neutral-600">接口地址</label>
            <Input id="cfg-url" value={settings.base_url} onChange={(e) => updateSettings({ base_url: e.target.value })} placeholder="https://api.openai.com/v1" />
          </div>
          <div className="flex flex-col gap-1.5">
            <label htmlFor="cfg-key" className="text-xs font-medium text-neutral-600">API Key</label>
            <div className="relative">
              <Input id="cfg-key" type={showApiKey ? 'text' : 'password'} value={apiKeyInput} onChange={(e) => setApiKeyInput(e.target.value)} placeholder="输入 API Key" />
              <button type="button" className="absolute right-3 top-1/2 -translate-y-1/2 text-neutral-400 hover:text-neutral-600" onClick={() => setShowApiKey(!showApiKey)}>
                {showApiKey ? <EyeOff className="w-4 h-4" /> : <Eye className="w-4 h-4" />}
              </button>
            </div>
          </div>
          <div className="grid grid-cols-2 gap-3">
            <div className="flex flex-col gap-1.5">
              <label htmlFor="cfg-pwd" className="text-xs font-medium text-neutral-600">保护密码</label>
              <Input id="cfg-pwd" type="password" value={password} onChange={(e) => setPassword(e.target.value)} placeholder="至少 8 位" />
              {!isPasswordStrong(password) && password.length > 0 && (
                <p className="text-xs text-amber-600">需包含大写、小写字母和数字</p>
              )}
            </div>
            <div className="flex flex-col gap-1.5">
              <label htmlFor="cfg-pwd-confirm" className="text-xs font-medium text-neutral-600">确认密码</label>
              <Input id="cfg-pwd-confirm" type="password" value={passwordConfirm} onChange={(e) => setPasswordConfirm(e.target.value)} placeholder="再次输入密码" />
              {password && password !== passwordConfirm && passwordConfirm.length > 0 && (
                <p className="text-xs text-red-500">密码不一致</p>
              )}
            </div>
          </div>
          <Button
            onClick={handleFirstTimeSave}
            disabled={!firstConfigName.trim() || !apiKeyInput.trim() || !password || !passwordConfirm || password !== passwordConfirm}
            className="self-start"
          >
            <KeyRound className="w-4 h-4 mr-2" /> 保存配置
          </Button>
        </div>
      </div>
    )
  }

  // Mode B: Locked
  if (lockStatus.locked) {
    return (
      <div className="flex flex-col gap-4">
        {renderMessage()}
        <div className="border border-amber-200 bg-amber-50/50 rounded-lg p-4 flex flex-col gap-3">
          <div className="flex items-center gap-2 text-amber-800">
            <Lock className="w-4 h-4" />
            <h4 className="text-sm font-medium">API Key 已加密</h4>
          </div>
          <p className="text-sm text-neutral-600">输入密码解锁后才能查看和修改配置</p>
          <div className="flex gap-3 items-center">
            <Input type="password" value={password} onChange={(e) => setPassword(e.target.value)} placeholder="输入密码解锁" className="flex-1" />
            <Button onClick={handleUnlock}><Unlock className="w-4 h-4 mr-2" /> 解锁</Button>
          </div>
        </div>
        {modelConfigs.length > 0 && (
          <div className="flex flex-col gap-2">
            <h4 className="text-sm font-medium text-neutral-700">已保存的配置</h4>
            {modelConfigs.map((cfg) => (
              <div key={cfg.id} className="flex items-center justify-between p-3 border border-neutral-200 rounded-lg bg-neutral-50/50">
                <div className="flex items-center gap-3">
                  {activeConfigId === cfg.id && <Check className="w-4 h-4 text-blue-600" />}
                  <div className="flex flex-col">
                    <span className="text-sm font-medium text-neutral-800">{cfg.name}</span>
                    <span className="text-xs text-neutral-500">{cfg.base_url}</span>
                  </div>
                </div>
                <span className="text-xs text-neutral-400 bg-neutral-100 px-2 py-1 rounded">{cfg.model_name}</span>
              </div>
            ))}
          </div>
        )}
      </div>
    )
  }

  // Mode C: Unlocked
  return (
    <div className="flex flex-col gap-4">
      {renderMessage()}

      <div className="flex flex-col gap-3">
        <div className="flex items-center justify-between">
          <h4 className="text-sm font-medium text-neutral-700">模型配置</h4>
          <div className="flex gap-2">
            {!showAddConfig && (
              <Button onClick={() => setShowAddConfig(true)} variant="outline" size="sm">
                <Plus className="w-3.5 h-3.5 mr-1.5" /> 新增配置
              </Button>
            )}
          </div>
        </div>

        {showAddConfig && (
          <div className="border border-blue-200 bg-blue-50/50 rounded-lg p-4 flex flex-col gap-3">
            <h5 className="text-xs font-medium text-blue-800">新增配置</h5>
            <div className="grid grid-cols-2 gap-3">
              <div className="flex flex-col gap-1.5">
                <label htmlFor="new-name" className="text-xs text-neutral-600">名称</label>
                <Input id="new-name" value={newConfigName} onChange={(e) => setNewConfigName(e.target.value)} placeholder="如：Claude" />
              </div>
              <div className="flex flex-col gap-1.5">
                <label htmlFor="new-model" className="text-xs text-neutral-600">模型</label>
                <Input id="new-model" value={newConfigModel} onChange={(e) => setNewConfigModel(e.target.value)} placeholder="claude-3" />
              </div>
            </div>
            <div className="flex flex-col gap-1.5">
              <label htmlFor="new-url" className="text-xs text-neutral-600">接口地址</label>
              <Input id="new-url" value={newConfigBaseUrl} onChange={(e) => setNewConfigBaseUrl(e.target.value)} placeholder="https://api.anthropic.com/v1" />
            </div>
            <div className="flex flex-col gap-1.5">
              <label htmlFor="new-key" className="text-xs text-neutral-600">API Key</label>
              <Input id="new-key" type="password" value={newConfigKey} onChange={(e) => setNewConfigKey(e.target.value)} placeholder="输入 API Key" />
            </div>
            <div className="grid grid-cols-2 gap-3">
              <div className="flex flex-col gap-1.5">
                <label htmlFor="new-pwd" className="text-xs text-neutral-600">保护密码</label>
                <Input id="new-pwd" type="password" value={newConfigPassword} onChange={(e) => setNewConfigPassword(e.target.value)} placeholder="至少 8 位" />
                {!isPasswordStrong(newConfigPassword) && newConfigPassword.length > 0 && (
                  <p className="text-xs text-amber-600">需包含大写、小写字母和数字</p>
                )}
              </div>
              <div className="flex flex-col gap-1.5">
                <label htmlFor="new-pwd-confirm" className="text-xs text-neutral-600">确认密码</label>
                <Input id="new-pwd-confirm" type="password" value={newConfigPasswordConfirm} onChange={(e) => setNewConfigPasswordConfirm(e.target.value)} placeholder="再次输入" />
                {newConfigPassword && newConfigPassword !== newConfigPasswordConfirm && newConfigPasswordConfirm.length > 0 && (
                  <p className="text-xs text-red-500">密码不一致</p>
                )}
              </div>
            </div>
            <div className="flex gap-2">
              <Button
                onClick={handleAddConfig}
                size="sm"
                disabled={!newConfigName.trim() || !newConfigBaseUrl.trim() || !newConfigModel.trim() || !newConfigKey.trim() || !isPasswordStrong(newConfigPassword) || newConfigPassword !== newConfigPasswordConfirm}
              >
                <Plus className="w-3.5 h-3.5 mr-1.5" /> 保存
              </Button>
              <Button onClick={() => setShowAddConfig(false)} variant="ghost" size="sm">取消</Button>
            </div>
          </div>
        )}

        {modelConfigs.map((cfg) => (
          <div
            key={cfg.id}
            className={
              'flex items-center justify-between p-4 rounded-lg border transition-colors cursor-pointer ' +
              (activeConfigId === cfg.id
                ? 'border-blue-300 bg-blue-50/30 shadow-sm'
                : 'border-neutral-200 hover:border-neutral-300 bg-white')
            }
            onClick={() => handleSetActive(cfg.id)}
          >
            <div className="flex items-center gap-3">
              <div className={'w-4 h-4 rounded-full border-2 flex items-center justify-center ' + (activeConfigId === cfg.id ? 'border-blue-500' : 'border-neutral-300')}>
                {activeConfigId === cfg.id && <div className="w-2 h-2 rounded-full bg-blue-500" />}
              </div>
              <div className="flex flex-col">
                <span className="text-sm font-medium text-neutral-800">{cfg.name}</span>
                <span className="text-xs text-neutral-500">{cfg.base_url}</span>
              </div>
            </div>
            <div className="flex items-center gap-2">
              <span className="text-xs bg-neutral-100 text-neutral-600 px-2 py-1 rounded font-mono">{cfg.model_name}</span>
              {deletingId === cfg.id ? (
                <div className="flex gap-1">
                  <Button variant="ghost" size="sm" className="text-red-500 h-7 px-2" onClick={() => handleDeleteConfig(cfg.id)}>确认删除</Button>
                  <Button variant="ghost" size="sm" className="h-7 px-2" onClick={() => setDeletingId(null)}>取消</Button>
                </div>
              ) : (
                <Button variant="ghost" size="icon" className="w-7 h-7 text-neutral-400 hover:text-red-500" onClick={() => setDeletingId(cfg.id)}>
                  <Trash2 className="w-3.5 h-3.5" />
                </Button>
              )}
            </div>
          </div>
        ))}
      </div>

      <div className="border-t border-neutral-100 pt-4 flex flex-col gap-3">
        <h4 className="text-sm font-medium text-neutral-700">修改当前密钥</h4>
        <div className="flex flex-col gap-1.5">
          <label htmlFor="edit-key" className="text-xs text-neutral-600">新 API Key</label>
          <div className="relative">
            <Input id="edit-key" type={showApiKey ? 'text' : 'password'} value={apiKeyInput} onChange={(e) => setApiKeyInput(e.target.value)} placeholder="输入新 API Key 以替换" />
            <button type="button" className="absolute right-3 top-1/2 -translate-y-1/2 text-neutral-400 hover:text-neutral-600" onClick={() => setShowApiKey(!showApiKey)}>
              {showApiKey ? <EyeOff className="w-4 h-4" /> : <Eye className="w-4 h-4" />}
            </button>
          </div>
        </div>
        <div className="grid grid-cols-2 gap-3">
          <div className="flex flex-col gap-1.5">
            <label htmlFor="edit-pwd" className="text-xs text-neutral-600">保护密码</label>
            <Input id="edit-pwd" type="password" value={password} onChange={(e) => setPassword(e.target.value)} placeholder="输入密码" />
            {!isPasswordStrong(password) && password.length > 0 && (
              <p className="text-xs text-amber-600">需包含大写、小写字母和数字</p>
            )}
          </div>
          <div className="flex flex-col gap-1.5">
            <label htmlFor="edit-pwd-confirm" className="text-xs text-neutral-600">确认密码</label>
            <Input id="edit-pwd-confirm" type="password" value={passwordConfirm} onChange={(e) => setPasswordConfirm(e.target.value)} placeholder="再次输入" />
            {password && password !== passwordConfirm && passwordConfirm.length > 0 && (
              <p className="text-xs text-red-500">密码不一致</p>
            )}
          </div>
        </div>
        <div className="flex gap-2 flex-wrap">
          <Button onClick={handleSaveCurrentKey} size="sm" disabled={!apiKeyInput.trim() || !password || !passwordConfirm || password !== passwordConfirm}>
            <KeyRound className="w-3.5 h-3.5 mr-1.5" /> 修改并重新加密
          </Button>
          <Button onClick={handleClearApiKey} variant="outline" size="sm">
            <Trash2 className="w-3.5 h-3.5 mr-1.5" /> 清除
          </Button>
          <Button onClick={handleLock} variant="outline" size="sm">
            <Lock className="w-3.5 h-3.5 mr-1.5" /> 锁定
          </Button>
        </div>
      </div>
    </div>
  )
}
