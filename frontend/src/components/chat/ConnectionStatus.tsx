import { RefreshCw } from 'lucide-react'

interface ConnectionStatusProps {
  connected: boolean
  reconnecting: boolean
  reconnectAttempts: number
  maxReconnectAttempts: number
  reconnectExhausted: boolean
  onManualReconnect: () => void
}

export function ConnectionStatus({
  connected,
  reconnecting,
  reconnectAttempts,
  maxReconnectAttempts,
  reconnectExhausted,
  onManualReconnect,
}: ConnectionStatusProps) {
  if (connected && !reconnecting) {
    return null
  }

  if (reconnecting) {
    return (
      <div className="flex items-center gap-2 px-4 py-2 text-sm text-yellow-700 bg-yellow-50 border-t border-yellow-100">
        <span className="w-2 h-2 rounded-full bg-yellow-400 animate-pulse" />
        <span>
          正在重连 ({reconnectAttempts}/{maxReconnectAttempts})...
        </span>
      </div>
    )
  }

  if (reconnectExhausted) {
    return (
      <div className="flex items-center gap-2 px-4 py-2 text-sm text-red-700 bg-red-50 border-t border-red-100">
        <span className="w-2 h-2 rounded-full bg-red-400" />
        <span className="flex-1">连接已断开</span>
        <button
          onClick={onManualReconnect}
          className="flex items-center gap-1 px-2 py-1 text-xs bg-red-100 hover:bg-red-200 rounded"
        >
          <RefreshCw className="w-3 h-3" />
          立即重连
        </button>
      </div>
    )
  }

  return null
}
