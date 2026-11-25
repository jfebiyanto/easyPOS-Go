import { readFileSync } from 'node:fs'
import { resolve } from 'node:path'
import { describe, expect, it } from 'vitest'
import { parsePref, resolveTheme } from './theme'

describe('resolveTheme', () => {
  it('mengikuti sistem secara default', () => {
    expect(resolveTheme('system', true)).toBe('dark')
    expect(resolveTheme('system', false)).toBe('light')
  })
  it('pilihan eksplisit mengalahkan sistem', () => {
    expect(resolveTheme('light', true)).toBe('light')
    expect(resolveTheme('dark', false)).toBe('dark')
  })
  it('parsePref: nilai asing atau kosong = system', () => {
    expect(parsePref(null)).toBe('system')
    expect(parsePref('ungu')).toBe('system')
    expect(parsePref('dark')).toBe('dark')
    expect(parsePref('light')).toBe('light')
  })
})

// Kontras dihitung dari token sebenarnya di theme.css.
const css = readFileSync(resolve(__dirname, 'theme.css'), 'utf8')
function tokens(selector: string): Record<string, string> {
  const block = css.split(selector)[1].split('}')[0]
  return Object.fromEntries([...block.matchAll(/--([\w-]+):\s*(#[0-9A-Fa-f]{6})/g)].map((m) => [m[1], m[2]]))
}
function lum(hex: string): number {
  const c = [1, 3, 5]
    .map((i) => parseInt(hex.slice(i, i + 2), 16) / 255)
    .map((v) => (v <= 0.03928 ? v / 12.92 : ((v + 0.055) / 1.055) ** 2.4))
  return 0.2126 * c[0] + 0.7152 * c[1] + 0.0722 * c[2]
}
function contrast(a: string, b: string): number {
  const [hi, lo] = [lum(a), lum(b)].sort((x, y) => y - x)
  return (hi + 0.05) / (lo + 0.05)
}

const pairs: [string, string][] = [
  ['text', 'bg'], ['text', 'surface'], ['text', 'surface-2'],
  ['text-muted', 'bg'], ['text-muted', 'surface'], ['text-muted', 'surface-2'],
  ['on-primary', 'primary'], ['on-primary', 'primary-hover'],
  ['primary-text', 'bg'], ['primary-text', 'surface'], ['primary-text', 'primary-soft'],
  ['warn', 'warn-soft'], ['warn', 'surface'], ['danger', 'danger-soft'], ['danger', 'surface'],
]

describe.each([
  ['terang', ':root {'],
  ['gelap', ':root[data-theme="dark"] {'],
])('kontras tema %s >= 4.5:1', (_name, selector) => {
  const t = tokens(selector)
  it.each(pairs)('%s di atas %s', (fg, bg) => {
    expect(contrast(t[fg], t[bg])).toBeGreaterThanOrEqual(4.5)
  })
  it('tidak ada warna hijau', () => {
    for (const [k, hex] of Object.entries(t)) {
      const [r, g, b] = [1, 3, 5].map((i) => parseInt(hex.slice(i, i + 2), 16))
      expect(g > r + 20 && g > b + 20, `${k} ${hex} terlalu hijau`).toBe(false)
    }
  })
})
