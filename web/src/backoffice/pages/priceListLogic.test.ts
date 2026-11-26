import { describe, expect, it } from 'vitest'
import { buildPayload, initialValues, tabNames } from '../../shared/ui/crudLogic'
import { draftFromItem, emptyDraft, itemPayload, itemUnitCode, minQtyError, perBaseText, validityText } from './priceListLogic'
import { priceListFields } from './pricelists'

const item = { id: 'i1', product_id: 'p1', unit_id: 'dus', unit: { id: 'dus', code: 'DUS' }, min_qty: 5, price: '10800.00', price_per_base: '900.00', factor: '12.000', product: { unit: { id: 'pcs', code: 'PCS' } } }
const baseItem = { ...item, unit_id: null, unit: null, price: '900.00' }

describe('form price list', () => {
  it('tanpa tab; nilai awal tambah: aktif, bukan default, tanggal kosong', () => {
    expect(tabNames(priceListFields)).toEqual([])
    expect(initialValues(priceListFields, null)).toMatchObject({ code: '', name: '', is_default: false, is_active: true, store_id: '', effective_from: '' })
  })
  it('payload: gerai/tanggal kosong jadi null, kode dan nama tetap string', () => {
    const p = buildPayload(priceListFields, initialValues(priceListFields, null), true)
    expect(p).toMatchObject({ code: '', name: '', store_id: null, effective_from: null, effective_until: null, is_default: false, is_active: true })
  })
})

describe('masa berlaku', () => {
  it('semua kombinasi', () => {
    expect(validityText(null, null)).toBe('Selalu')
    expect(validityText('2026-01-01', null)).toBe('Mulai 2026-01-01')
    expect(validityText(null, '2026-12-31')).toBe('s/d 2026-12-31')
    expect(validityText('2026-01-01', '2026-12-31')).toBe('2026-01-01 s/d 2026-12-31')
  })
})

describe('item', () => {
  it('min qty: bilangan bulat >= 1', () => {
    for (const ok of ['1', '10', ' 3 ']) expect(minQtyError(ok)).toBe('')
    for (const bad of ['', '0', '1.5', '-2', 'a']) expect(minQtyError(bad)).not.toBe('')
  })
  it('payload tambah membawa produk; edit tidak; satuan kosong = null', () => {
    const d = { ...emptyDraft(), productId: 'p1', price: '900' }
    expect(itemPayload(d, true)).toEqual({ product_id: 'p1', unit_id: null, min_qty: 1, price: '900' })
    expect(itemPayload({ ...d, unitId: 'dus', minQty: '5' }, false)).toEqual({ unit_id: 'dus', min_qty: 5, price: '900' })
  })
  it('draft dari baris yang ada', () => {
    expect(draftFromItem(item)).toEqual({ productId: 'p1', unitId: 'dus', minQty: '5', price: '10800.00' })
    expect(draftFromItem(baseItem).unitId).toBe('')
  })
  it('kode satuan: khusus atau satuan dasar produk', () => {
    expect(itemUnitCode(item)).toBe('DUS')
    expect(itemUnitCode(baseItem)).toBe('PCS')
  })
  it('harga per satuan dasar hanya untuk satuan non-dasar', () => {
    expect(perBaseText(item, (s) => s)).toBe('900.00 / PCS')
    expect(perBaseText(baseItem, (s) => s)).toBe('—')
  })
})
