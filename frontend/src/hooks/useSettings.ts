import { useState, useEffect, useRef } from 'react'
import type { Settings } from '@/types'
import { getSettings, saveSettings } from '@/lib/api'

export function DefaultSettings(): Settings {
  return {
    model: 'gpt-4',
    agent_id: 'default',
    context_window: 8192,
    working_directory: '',
    language: 'zh-CN',
  }
}

interface UseSettingsReturn {
  settings: Settings
  loading: boolean
  error: string | null
  updateSettings: (patch: Partial<Settings>) => void
}

export function useSettings(): UseSettingsReturn {
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

  const updateSettings = (patch: Partial<Settings>) => {
    const newSettings = { ...settingsRef.current, ...patch }
    settingsRef.current = newSettings
    setSettings(newSettings)
    if (debounceRef.current) clearTimeout(debounceRef.current)
    debounceRef.current = setTimeout(() => {
      saveSettings(newSettings).catch((e: Error) => setError(`保存设置失败: ${e.message}`))
    }, 300)
  }

  return { settings, loading, error, updateSettings }
}
