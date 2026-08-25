import { Camera } from 'lucide-react'
import { useSnapshot } from '@/hooks/useSnapshot'

export function StatusBar() {
  const isElectron = typeof window !== 'undefined' && window.flowPartner
  const snapshot = useSnapshot()
  const status = snapshot.status

  const snapshotText = () => {
    if (!status) return null
    if (status.phase === 'snapshotting') return '快照中...'
    if (status.phase === 'error') return '快照异常'
    if (status.last_at) return `快照 ${status.last_snapshot_id || ''}`
    return null
  }

  return (
    <div className="h-6 flex items-center px-3 border-t border-neutral-200 bg-neutral-50 text-xs text-neutral-500 shrink-0">
      <span className="mr-auto">{isElectron ? '桌面版 · FlowPartner' : '浏览器运行 · 仅 UI 预览'}</span>
      {snapshotText() && (
        <span
          className={
            'flex items-center gap-1.5 px-2 rounded ' +
            (status?.phase === 'error'
              ? 'text-red-600'
              : status?.phase === 'snapshotting'
                ? 'text-blue-600'
                : 'text-green-600')
          }
        >
          <Camera className="w-3 h-3" />
          {snapshotText()}
        </span>
      )}
    </div>
  )
}
