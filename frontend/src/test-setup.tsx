import { vi, afterEach } from 'vitest'
import { cleanup } from '@testing-library/react'
import '@testing-library/jest-dom'

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

afterEach(cleanup)
