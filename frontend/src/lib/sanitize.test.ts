import { describe, it, expect } from 'vitest'
import { esc } from './sanitize'

describe('esc()', () => {
  it('escapes ampersands', () => {
    expect(esc('a&b')).toBe('a&amp;b')
  })

  it('escapes angle brackets', () => {
    expect(esc('<script>')).toBe('&lt;script&gt;')
  })

  it('escapes double quotes', () => {
    expect(esc('he said "hi"')).toBe('he said &quot;hi&quot;')
  })

  it('escapes single quotes', () => {
    expect(esc("it's")).toBe('it&#39;s')
  })

  it('escapes forward slashes', () => {
    expect(esc('a/b')).toBe('a&#x2F;b')
  })

  it('escapes multiple special chars in one string', () => {
    const input = '<div class="x">Hello & Goodbye</div>'
    const result = esc(input)
    expect(result).not.toContain('<')
    expect(result).not.toContain('>')
    expect(result).not.toContain('"')
    expect(result).toContain('&amp;')
  })

  it('returns empty string unchanged', () => {
    expect(esc('')).toBe('')
  })

  it('returns plain text unchanged', () => {
    expect(esc('hello world')).toBe('hello world')
  })
})
