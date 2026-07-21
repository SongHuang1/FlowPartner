import { describe, it, expect } from 'vitest'
import { cn } from './utils'

describe('cn', () => {
  it('merges class names into a single string', () => {
    const result = cn('foo', 'bar')
    expect(result).toBe('foo bar')
  })

  it('handles conditional classes', () => {
    const isActive = true
    const isHidden = false
    const result = cn('base', isActive && 'active', isHidden && 'hidden')
    expect(result).toBe('base active')
  })

  it('handles null and undefined values', () => {
    const result = cn('base', null, undefined, 'valid')
    expect(result).toBe('base valid')
  })

  it('handles empty string', () => {
    const result = cn('')
    expect(result).toBe('')
  })

  it('handles no arguments', () => {
    const result = cn()
    expect(result).toBe('')
  })

  it('deduplicates conflicting Tailwind classes (last wins)', () => {
    const result = cn('p-4', 'p-2')
    expect(result).toBe('p-2')
  })

  it('merges non-conflicting Tailwind classes', () => {
    const result = cn('p-4', 'm-2')
    expect(result).toBe('p-4 m-2')
  })

  it('handles array syntax', () => {
    const result = cn(['foo', 'bar'])
    expect(result).toBe('foo bar')
  })

  it('handles object syntax', () => {
    const result = cn({ foo: true, bar: false, baz: true })
    expect(result).toBe('foo baz')
  })

  it('resolves Tailwind class conflicts with same property', () => {
    const result = cn('text-red-500', 'text-blue-500')
    expect(result).toBe('text-blue-500')
  })

  it('handles mixed input types', () => {
    const result = cn('base', { conditional: true }, ['array-class'], null)
    expect(result).toBe('base conditional array-class')
  })

  it('handles Tailwind utility classes with modifiers', () => {
    const result = cn('hover:bg-red-500', 'hover:bg-blue-500')
    expect(result).toBe('hover:bg-blue-500')
  })
})
