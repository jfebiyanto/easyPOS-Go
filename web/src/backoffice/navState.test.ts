import { describe, expect, it } from 'vitest'
import { parseCollapsed, toggle } from './navState'

describe('navState', () => {
  it('parseCollapsed: kosong, rusak, atau bukan array = semua terbuka', () => {
    expect(parseCollapsed(null)).toEqual([])
    expect(parseCollapsed('{{')).toEqual([])
    expect(parseCollapsed('{"a":1}')).toEqual([])
    expect(parseCollapsed('["Stok",1,"Katalog"]')).toEqual(['Stok', 'Katalog'])
  })
  it('toggle menambah lalu menghapus', () => {
    expect(toggle([], 'Stok')).toEqual(['Stok'])
    expect(toggle(['Stok', 'Katalog'], 'Stok')).toEqual(['Katalog'])
  })
})
