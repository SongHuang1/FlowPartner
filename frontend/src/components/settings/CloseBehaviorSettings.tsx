import { useSettings } from '@/hooks/useSettings'
import { useEffect } from 'react'

export function CloseBehaviorSettings() {
  const { settings, updateSettings } = useSettings()

  useEffect(() => {
    window.flowPartner.updateCloseBehavior(settings.close_behavior, settings.close_remembered)
  }, [settings.close_behavior, settings.close_remembered])

  const handleChange = (value: 'ask' | 'minimize' | 'quit') => {
    const remembered = value !== 'ask'
    updateSettings({
      close_behavior: value,
      close_remembered: remembered,
    })
    window.flowPartner.updateCloseBehavior(value, remembered)
  }

  const options: { value: 'ask' | 'minimize' | 'quit'; label: string; desc: string }[] = [
    { value: 'ask', label: '每次询问', desc: '关闭时弹出选择框' },
    { value: 'minimize', label: '最小化到托盘', desc: '关闭窗口时后台运行' },
    { value: 'quit', label: '完全退出', desc: '关闭窗口时结束程序' },
  ]

  return (
    <div className="flex flex-col gap-4">
      <div className="flex flex-col gap-3">
        <h4 className="text-sm font-medium text-neutral-700">关闭行为</h4>
        <p className="text-xs text-neutral-500">选择点击关闭窗口按钮后的默认行为</p>
        <div className="flex flex-col gap-2 mt-1">
          {options.map(({ value, label, desc }) => (
            <label
              key={value}
              className={
                'flex items-start gap-4 p-4 rounded-lg border transition-colors cursor-pointer ' +
                (settings.close_behavior === value
                  ? 'border-blue-300 bg-blue-50/30 shadow-sm'
                  : 'border-neutral-200 hover:border-neutral-300 hover:bg-neutral-50')
              }
              onClick={() => handleChange(value)}
            >
              <input
                type="radio"
                name="close-behavior"
                checked={settings.close_behavior === value}
                onChange={() => handleChange(value)}
                className="mt-0.5 accent-blue-500"
              />
              <div className="flex flex-col gap-1">
                <span className="text-sm font-medium text-neutral-700">{label}</span>
                <span className="text-xs text-neutral-500">{desc}</span>
              </div>
            </label>
          ))}
        </div>
      </div>
    </div>
  )
}
