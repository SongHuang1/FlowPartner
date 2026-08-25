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
    platform: 'win32',
    getVersion: vi.fn().mockResolvedValue('0.0.0'),
    onSystemLock: vi.fn().mockReturnValue(noop),
    onSystemFocus: vi.fn().mockReturnValue(noop),
    fetchBackendPort: vi.fn().mockResolvedValue(8080),
    onCloseAction: vi.fn().mockReturnValue(noop),
    sendCloseAction: noop,
    updateCloseBehavior: noop,
    openExternal: vi.fn().mockResolvedValue(undefined),
    selectFolder: vi.fn().mockResolvedValue(null),
  },
})

afterEach(cleanup)
