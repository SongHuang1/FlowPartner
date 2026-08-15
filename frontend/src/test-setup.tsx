import { vi, afterEach } from 'vitest'
import { cleanup } from '@testing-library/react'
import '@testing-library/jest-dom'
import { LockProvider } from './hooks/useLock'
import { SettingsProvider } from './hooks/useSettings'

if (!HTMLElement.prototype.scrollTo) {
  HTMLElement.prototype.scrollTo = vi.fn()
}

const noop = () => {}

Object.defineProperty(window, 'flowPartner', {
  writable: true,
  value: {
    fetchBackendPort: vi.fn().mockResolvedValue(8080),
    onBackendPortChanged: vi.fn().mockReturnValue(noop),
    onCloseAction: vi.fn().mockReturnValue(noop),
    sendCloseAction: noop,
    updateCloseBehavior: noop,
    onLockStatusChanged: vi.fn().mockReturnValue(noop),
    onBackendReady: vi.fn().mockReturnValue(noop),
  },
})

function AllProviders({ children }: { children: React.ReactNode }) {
  return (
    <SettingsProvider>
      <LockProvider>
        {children}
      </LockProvider>
    </SettingsProvider>
  )
}

afterEach(cleanup)

export { AllProviders }
