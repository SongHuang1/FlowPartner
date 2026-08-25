import { useState } from 'react'
import { HardDrive, FolderOpen, Camera, RotateCcw, Shield, FileWarning, X } from 'lucide-react'
import { Button } from '@/components/ui/button'
import { useSettings } from '@/hooks/useSettings'
import { useSnapshot } from '@/hooks/useSnapshot'
import type { SnapshotManifest, SnapshotDetail } from '@/types'

function formatBytes(bytes: number): string {
  if (bytes === 0) return '0 B'
  const units = ['B', 'KB', 'MB', 'GB', 'TB']
  let value = bytes
  let i = 0
  while (value >= 1024 && i < units.length - 1) {
    value /= 1024
    i++
  }
  return `${value.toFixed(1)} ${units[i]}`
}

function formatDate(iso: string): string {
  const d = new Date(iso)
  return d.toLocaleString('zh-CN', { hour12: false })
}

const reasonLabels: Record<string, string> = {
  debounce: '自动（变更后）',
  ticker: '定时',
  lock: '锁屏',
  manual: '手动',
  prerestore: '还原前自动备份',
}

export function SnapshotSettings() {
  const { settings, updateSettings } = useSettings()
  const snapshot = useSnapshot()
  const [pickingDir, setPickingDir] = useState(false)
  const [confirmRestore, setConfirmRestore] = useState<SnapshotManifest | null>(null)
  const [detail, setDetail] = useState<SnapshotDetail | null>(null)
  const [deleteExtras, setDeleteExtras] = useState(true)
  const [detailLoading, setDetailLoading] = useState(false)
  const [localError, setLocalError] = useState<string | null>(null)

  const enabled = settings.snapshot_enabled
  const status = snapshot.status

  const pickDir = async () => {
    setPickingDir(true)
    try {
      const dir = await window.flowPartner.selectFolder()
      if (dir) {
        updateSettings({ snapshot_dir: dir })
      }
    } catch {
      setLocalError('选择目录失败')
    } finally {
      setPickingDir(false)
    }
  }

  const openRestoreConfirm = async (s: SnapshotManifest) => {
    setConfirmRestore(s)
    setDetail(null)
    setDeleteExtras(true)
    setDetailLoading(true)
    try {
      const d = await snapshot.getDetail(s.snapshot_id)
      setDetail(d)
    } finally {
      setDetailLoading(false)
    }
  }

  const handleRestore = () => {
    if (!confirmRestore) return
    snapshot.restore(confirmRestore.snapshot_id, deleteExtras)
    setConfirmRestore(null)
    setDetail(null)
  }

  const toggleEnabled = (next: boolean) => {
    if (next && !settings.snapshot_dir.trim()) {
      setLocalError('请先选择快照储存目录')
      return
    }
    setLocalError(null)
    updateSettings({ snapshot_enabled: next })
  }

  const phaseLabel = () => {
    if (!status) return '未初始化'
    switch (status.phase) {
      case 'snapshotting': return '正在创建快照...'
      case 'error': return status.error || '快照出错'
      default: return status.queued ? '快照排队中' : '自动快照已开启'
    }
  }

  return (
    <div className="flex flex-col gap-4">
      {localError && (
        <div className="text-sm text-red-600 bg-red-50 px-4 py-3 rounded-lg border border-red-100">{localError}</div>
      )}

      {snapshot.error && (
        <div className="text-sm text-red-600 bg-red-50 px-4 py-3 rounded-lg border border-red-100">{snapshot.error}</div>
      )}

      <div className="border border-neutral-200 rounded-lg p-4 flex flex-col gap-3">
        <div className="flex items-center justify-between">
          <div className="flex items-center gap-2">
            <HardDrive className="w-4 h-4 text-neutral-500" />
            <h4 className="text-sm font-medium text-neutral-800">本地快照（自动保存）</h4>
          </div>
          <button
            role="switch"
            aria-checked={enabled}
            onClick={() => toggleEnabled(!enabled)}
            className={
              'relative shrink-0 w-11 h-6 rounded-full transition-colors ' +
              (enabled ? 'bg-blue-600' : 'bg-neutral-300')
            }
          >
            <span className={'absolute top-0.5 left-0.5 w-5 h-5 rounded-full bg-white shadow transition-transform ' + (enabled ? 'translate-x-[22px]' : 'translate-x-0')} />
          </button>
        </div>

        <div className="flex flex-col gap-1.5">
          <label className="text-xs font-medium text-neutral-600">快照储存目录</label>
          <div className="flex gap-2">
            <input
              value={settings.snapshot_dir}
              onChange={(e) => updateSettings({ snapshot_dir: e.target.value })}
              placeholder="选择快照保存位置（必须为绝对路径）"
              className="flex-1 px-3 py-2 text-sm border border-neutral-300 rounded-lg focus:outline-none focus:ring-2 focus:ring-blue-500"
            />
            <Button variant="outline" size="sm" onClick={pickDir} disabled={pickingDir}>
              <FolderOpen className="w-3.5 h-3.5 mr-1.5" />
              {pickingDir ? '选择中...' : '选择目录'}
            </Button>
          </div>
        </div>

        <label className="flex items-center gap-2 text-sm text-neutral-600 cursor-pointer">
          <input
            type="checkbox"
            checked={settings.snapshot_include_secrets}
            onChange={(e) => updateSettings({ snapshot_include_secrets: e.target.checked })}
            className="accent-blue-600"
          />
          <span>快照中包含敏感文件（密钥、证书等，默认排除）</span>
        </label>

        {enabled && status && (
          <div className="text-xs text-neutral-500 bg-neutral-50 rounded-lg px-3 py-2 flex flex-col gap-1">
            <div className="flex items-center gap-2">
              <span className={status.phase === 'snapshotting' ? 'text-blue-600' : status.phase === 'error' ? 'text-red-600' : 'text-green-600'}>
                {phaseLabel()}
              </span>
              {status.last_at && <span className="text-neutral-400">· 上次：{formatDate(status.last_at)}</span>}
            </div>
            <div className="text-neutral-400">
              共 {status.count} 个快照，占用 {formatBytes(status.size_bytes)}
              {status.skipped_files > 0 && `，跳过 ${status.skipped_files} 个文件`}
            </div>
          </div>
        )}

        <div className="flex gap-2">
          <Button size="sm" onClick={snapshot.manualSnapshot} disabled={!enabled || status?.phase === 'snapshotting'}>
            <Camera className="w-3.5 h-3.5 mr-1.5" /> 立即创建快照
          </Button>
          <Button variant="outline" size="sm" onClick={snapshot.refreshList}>
            刷新
          </Button>
        </div>
      </div>

      {snapshot.messages.length > 0 && (
        <div className="flex flex-col gap-2">
          {snapshot.messages.map((m, i) => (
            <div
              key={i}
              className={
                'text-sm px-4 py-2 rounded-lg border flex items-start justify-between gap-2 ' +
                (m.type === 'error'
                  ? 'text-red-700 bg-red-50 border-red-100'
                  : m.type === 'warning'
                    ? 'text-amber-700 bg-amber-50 border-amber-100'
                    : 'text-green-700 bg-green-50 border-green-100')
              }
            >
              <span>{m.text}</span>
              <button onClick={snapshot.dismissMessages} className="text-neutral-400 hover:text-neutral-600 shrink-0">
                <X className="w-3.5 h-3.5" />
              </button>
            </div>
          ))}
        </div>
      )}

      <div className="flex flex-col gap-2">
        <div className="flex items-center justify-between">
          <h4 className="text-sm font-medium text-neutral-700">历史快照</h4>
          {snapshot.loading && <span className="text-xs text-neutral-400">加载中...</span>}
        </div>
        {!enabled ? (
          <p className="text-xs text-neutral-400">启用自动快照后可查看历史快照</p>
        ) : snapshot.snapshots.length === 0 ? (
          <p className="text-xs text-neutral-400">暂无快照。文件变更后将自动保存，也可点击"立即创建快照"。</p>
        ) : (
          <div className="flex flex-col gap-2">
            {snapshot.snapshots.map((s) => (
              <div key={s.snapshot_id} className="flex items-center justify-between p-3 border border-neutral-200 rounded-lg bg-neutral-50/50">
                <div className="flex flex-col gap-0.5">
                  <span className="text-sm font-medium text-neutral-800 font-mono">{s.snapshot_id}</span>
                  <span className="text-xs text-neutral-500">
                    {reasonLabels[s.reason] || s.reason} · {s.file_count} 个文件 · {formatBytes(s.total_size_bytes)}
                  </span>
                  <span className="text-xs text-neutral-400">{formatDate(s.created_at)}</span>
                </div>
                <Button variant="outline" size="sm" onClick={() => openRestoreConfirm(s)}>
                  <RotateCcw className="w-3.5 h-3.5 mr-1.5" /> 还原
                </Button>
              </div>
            ))}
          </div>
        )}
      </div>

      {confirmRestore && (
        <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/40" onClick={() => setConfirmRestore(null)}>
          <div className="bg-white rounded-xl shadow-2xl w-[480px] max-h-[85vh] flex flex-col overflow-hidden" onClick={(e) => e.stopPropagation()}>
            <div className="flex items-center justify-between px-5 py-3 border-b border-neutral-200 bg-neutral-50">
              <h3 className="text-sm font-semibold text-neutral-800 flex items-center gap-2">
                <Shield className="w-4 h-4 text-blue-600" /> 还原快照
              </h3>
              <button onClick={() => setConfirmRestore(null)} className="text-neutral-400 hover:text-neutral-600">
                <X className="w-4 h-4" />
              </button>
            </div>
            <div className="flex-1 overflow-y-auto p-5 flex flex-col gap-3 text-sm">
              <p className="text-neutral-700">
                将把快照 <span className="font-mono font-medium">{confirmRestore.snapshot_id}</span> 的文件写回工作区。
                还原前会自动对当前状态创建一份预快照，可随时回退。
              </p>
              <div className="bg-amber-50 border border-amber-200 rounded-lg px-3 py-2 text-amber-800 text-xs">
                如果还原后不满意，可在历史快照中选择还原前的预快照恢复原状。
              </div>
              {detailLoading ? (
                <p className="text-xs text-neutral-400">正在检查受保护文件...</p>
              ) : detail && detail.protected_files.length > 0 ? (
                <div className="flex flex-col gap-1.5">
                  <p className="text-xs font-medium text-neutral-600 flex items-center gap-1.5">
                    <FileWarning className="w-3.5 h-3.5 text-amber-500" />
                    以下文件已受保护，不会被删除：
                  </p>
                  <div className="max-h-40 overflow-y-auto flex flex-col gap-1">
                    {detail.protected_files.slice(0, 20).map((p, i) => (
                      <div key={i} className="text-xs text-neutral-500 flex items-center justify-between gap-2">
                        <span className="font-mono truncate">{p.path}</span>
                        <span className="shrink-0 text-neutral-400">
                          {p.type === 'secret' ? '敏感文件' : p.type === 'too_large' ? '超大文件' : '排除目录'}
                        </span>
                      </div>
                    ))}
                    {detail.protected_files.length > 20 && (
                      <span className="text-xs text-neutral-400">...共 {detail.protected_files.length} 项</span>
                    )}
                  </div>
                </div>
              ) : (
                <p className="text-xs text-neutral-400">未检测到受保护文件。</p>
              )}
              <label className="flex items-center gap-2 text-neutral-700 cursor-pointer">
                <input
                  type="checkbox"
                  checked={deleteExtras}
                  onChange={(e) => setDeleteExtras(e.target.checked)}
                  className="accent-blue-600"
                />
                <span>删除快照中不存在但当前存在的多余文件（受保护文件除外）</span>
              </label>
            </div>
            <div className="flex justify-end gap-2 px-5 py-3 border-t border-neutral-200 bg-neutral-50">
              <Button variant="ghost" onClick={() => setConfirmRestore(null)}>取消</Button>
              <Button onClick={handleRestore}>
                <RotateCcw className="w-4 h-4 mr-2" /> 确认还原
              </Button>
            </div>
          </div>
        </div>
      )}
    </div>
  )
}