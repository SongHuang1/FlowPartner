import { vi, afterEach } from 'vitest'
import { cleanup } from '@testing-library/react'
import '@testing-library/jest-dom'

if (!HTMLElement.prototype.scrollTo) {
  HTMLElement.prototype.scrollTo = vi.fn()
}

afterEach(() => {
  cleanup()
})

export {}
