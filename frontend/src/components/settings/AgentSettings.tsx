import { useSettings } from '@/hooks/useSettings'

export function AgentSettings() {
  const { settings, updateSettings } = useSettings()

  return (
    <div className="flex flex-col gap-4">
      <h3 className="text-sm font-medium text-neutral-700">Agent 配置</h3>

      <div className="flex flex-col gap-1">
        <label htmlFor="agent-system-prompt" className="text-xs font-medium text-neutral-600">系统提示词</label>
        <textarea
          id="agent-system-prompt"
          value={settings.system_prompt}
          onChange={(e) => updateSettings({ system_prompt: e.target.value })}
          placeholder="你是一个有帮助的 AI 助手。"
          rows={4}
          className="flex w-full rounded-md border border-neutral-200 bg-transparent px-3 py-2 text-sm shadow-sm placeholder:text-neutral-500 focus-visible:outline-none focus-visible:ring-1 focus-visible:ring-neutral-300"
        />
      </div>

      <div className="flex flex-col gap-1">
        <label htmlFor="agent-temperature" className="text-xs font-medium text-neutral-600">
          温度 ({settings.temperature.toFixed(1)})
        </label>
        <input
          id="agent-temperature"
          type="range"
          min="0"
          max="2"
          step="0.1"
          value={settings.temperature}
          onChange={(e) => updateSettings({ temperature: parseFloat(e.target.value) })}
          className="w-full"
        />
        <div className="flex justify-between text-xs text-neutral-400">
          <span>0.0 (精确)</span>
          <span>2.0 (创意)</span>
        </div>
      </div>
    </div>
  )
}
