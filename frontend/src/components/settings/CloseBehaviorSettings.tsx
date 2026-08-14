import { useSettings } from '@/hooks/useSettings'

export function CloseBehaviorSettings() {
  const { settings, updateSettings } = useSettings()

  const options: { value: 'ask' | 'minimize' | 'quit'; label: string; desc: string }[] = [
    { value: 'ask', label: '每次询问', desc: '关闭窗口时弹出选择' },
    { value: 'minimize', label: '最小化到托盘', desc: '后台继续运行' },
    { value: 'quit', label: '完全退出', desc: '结束程序' },
  ]

  return (
    <div className="flex flex-col gap-3">
      <h3 className="text-sm font-medium text-neutral-700">关闭行为</h3>
      <div className="flex flex-col gap-2">
        {options.map(({ value, label, desc }) => (
          <label
            key={value}
            className="flex items-start gap-3 p-2 rounded-md border border-neutral-200 cursor-pointer hover:bg-neutral-50"
          >
            <input
              type="radio"
              name="close-behavior"
              checked={settings.close_behavior === value}
              onChange={() => updateSettings({ close_behavior: value })}
              className="mt-0.5"
            />
            <div className="flex flex-col">
              <span className="text-sm text-neutral-700">{label}</span>
              <span className="text-xs text-neutral-500">{desc}</span>
            </div>
          </label>
        ))}
      </div>
    </div>
  )
}
