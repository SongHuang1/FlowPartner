import { vi } from 'vitest'
import '@testing-library/jest-dom'

if (!HTMLElement.prototype.scrollTo) {
  HTMLElement.prototype.scrollTo = vi.fn()
}

export {}
