import { createContext, useContext, useState, useEffect, useRef, useCallback } from 'react'
import type { Settings } from '@/types'
import { getSettings, saveSettings } from '@/lib/api'

export function DefaultSettings(): Settings {
  return {
    model: 'gpt-4',
    agent_id: 'default',
    context_window: 8192,
    working_directory: '',
    language: 'zh-CN',
    base_url: 'https://api.openai.com/v1',
    encrypted_api_key: '',
    model_name: 'gpt-4',
    system_prompt: '你是一个乐于助人的 AI 助手。',
    temperature: 0.7,
    close_behavior: 'ask',
    close_remembered: false,
    window_x: 100,
    window_y: 100,
    window_width: 1200,
    window_height: 800,
    sidebar_visible: true,
    sidebar_view: 'conversation',
    trash_dir: '',
    snapshot_dir: '',
    snapshot_enabled: false,
    snapshot_include_secrets: false,
    protocol_v2: true,
  }
}

interface SettingsContextValue {
  settings: Settings
  loading: boolean
  error: string | null
  updateSettings: (patch: Partial<Settings>) => void
  getCurrentSettings: () => Settings
  refreshSettings: () => Promise<void>
}

const SettingsContext = createContext<SettingsContextValue | null>(null)

export function SettingsProvider({ children }: { children: React.ReactNode }) {
  const [settings, setSettings] = useState<Settings>(DefaultSettings())
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)
  const debounceRef = useRef<ReturnType<typeof setTimeout> | null>(null)
  const settingsRef = useRef<Settings>(settings)

  useEffect(() => {
    getSettings()
      .then((s) => {
        settingsRef.current = s
        setSettings(s)
      })
      .catch((e: Error) => setError(e.message))
      .finally(() => setLoading(false))
  }, [])

  useEffect(() => {
    return () => {
      if (debounceRef.current) clearTimeout(debounceRef.current)
    }
  }, [])

  const updateSettings = useCallback((patch: Partial<Settings>) => {
    const newSettings = { ...settingsRef.current, ...patch }
    settingsRef.current = newSettings
    setSettings(newSettings)
    if (debounceRef.current) clearTimeout(debounceRef.current)
    debounceRef.current = setTimeout(() => {
      saveSettings(newSettings)
        .then((saved) => {
          settingsRef.current = saved
          setSettings(saved)
        })
        .catch((e: Error) => setError(`保存设置失败：${e.message}`))
    }, 300)
  }, [])

  const getCurrentSettings = useCallback(() => settingsRef.current, [])

  const refreshSettings = useCallback(async () => {
    if (debounceRef.current) {
      clearTimeout(debounceRef.current)
      debounceRef.current = null
    }
    try {
      const s = await getSettings()
      settingsRef.current = s
      setSettings(s)
    } catch (e) {
      setError(e instanceof Error ? e.message : '刷新设置失败')
    }
  }, [])

  return (
    <SettingsContext.Provider value={{ settings, loading, error, updateSettings, getCurrentSettings, refreshSettings }}>
      {children}
    </SettingsContext.Provider>
  )
}

export function useSettings(): SettingsContextValue {
  const ctx = useContext(SettingsContext)
  if (!ctx) {
    throw new Error('useSettings must be used within SettingsProvider')
  }
  return ctx
}
