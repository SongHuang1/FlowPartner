import { useEffect, useCallback } from 'react'
import { AlertTriangle } from 'lucide-react'
import { Button } from '@/components/ui/button'
import type { PermissionRequestPayload } from '@/types'

type PermissionDecision = 'allow' | 'allow_session' | 'deny'

interface PermissionDialogProps {
  request: PermissionRequestPayload
  onDecision: (decision: PermissionDecision) => void
}

export function PermissionDialog({ request, onDecision }: PermissionDialogProps) {
  const handleKeyDown = useCallback(
    (e: KeyboardEvent) => {
      if (e.key === 'Escape') {
        onDecision('deny')
      }
    },
    [onDecision],
  )

  useEffect(() => {
    document.addEventListener('keydown', handleKeyDown)
    return () => document.removeEventListener('keydown', handleKeyDown)
  }, [handleKeyDown])

  const hasSessionOption = request.scope_options?.includes('session')

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center" role="dialog" aria-modal="true" aria-labelledby="permission-dialog-title">
      <div
        className="fixed inset-0 bg-black/50"
        onClick={() => onDecision('deny')}
      />
      <div className="relative bg-white rounded-lg shadow-xl max-w-md w-full mx-4 p-6 z-10">
        <div className="flex items-center gap-3 mb-4">
          <div className="flex-shrink-0 w-10 h-10 rounded-full bg-amber-100 flex items-center justify-center">
            <AlertTriangle className="w-5 h-5 text-amber-600" />
          </div>
          <h2 id="permission-dialog-title" className="text-lg font-semibold text-neutral-900">权限申请</h2>
        </div>

        <div className="mb-6 space-y-2">
          <p className="text-sm text-neutral-700">
            Agent 想要执行以下操作：
          </p>
          <div className="bg-neutral-50 border border-neutral-200 rounded-md p-3 space-y-1">
            <p className="text-sm">
              <span className="font-medium text-neutral-600">操作：</span>
              <span className="text-neutral-900">{request.operation}文件</span>
            </p>
            <p className="text-sm">
              <span className="font-medium text-neutral-600">工具：</span>
              <span className="text-neutral-900 font-mono">{request.tool}</span>
            </p>
            <p className="text-sm break-all">
              <span className="font-medium text-neutral-600">路径：</span>
              <span className="text-neutral-900 font-mono text-xs">{request.path}</span>
            </p>
          </div>
        </div>

        <div className="flex flex-col gap-2 justify-end">
          {hasSessionOption && (
            <p className="text-xs text-neutral-500 text-center mb-1">
              "本次会话允许"将在同一会话内自动放行相同工具和路径，跨会话仍需授权。
            </p>
          )}
          <div className="flex gap-2 justify-end">
            <Button
              variant="outline"
              onClick={() => onDecision('deny')}
            >
              拒绝
            </Button>
            {hasSessionOption && (
              <Button
                variant="outline"
                onClick={() => onDecision('allow_session')}
              >
                本次会话允许
              </Button>
            )}
            <Button
              onClick={() => onDecision('allow')}
            >
              允许一次
            </Button>
          </div>
        </div>
      </div>
    </div>
  )
}
