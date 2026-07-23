import { useEffect } from 'react'

export function useWindowState() {
  useEffect(() => {
    if (window.flowPartner?.onSystemLock) {
      window.flowPartner.onSystemLock(() => {
        window.dispatchEvent(new CustomEvent('system-lock'))
      })
    }
  }, [])
}
