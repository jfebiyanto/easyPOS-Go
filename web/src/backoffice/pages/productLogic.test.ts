import { describe, expect, it } from 'vitest'
import { buildPayload, initialValues, tabNames, tabsWithErrors } from '../../shared/ui/crudLogic'
import { allowedUnitIds, barcodeRows, newBarcodeRow } from './productLogic'
import { productFields } from './products'

const detail = {
  id: 'p1', sku: 'SKU1', name: 'Mie', barcode: '8990001', base_price: '3500.00', manage_by: 'batch', track_stock: true, is_active: true,
  product_group_id: 'g1', brand_id: null, unit_id: 'pcs', uom_group_id: 'grp', sales_unit_id: 'dus', image_url: '/media/products/a.png',
  barcodes: [{ id: 'b1', code: '8990012', unit_id: 'dus', note: null, source: 'local' }, { id: 'b2', code: 'SAP1', unit_id: 'pcs', note: 'x', source: 'sap' }],
}

describe('form produk', () => {
  it('tab: Umum, Satuan & Barcode, Gambar', () => {
    expect(tabNames(productFields)).toEqual(['Umum', 'Satuan & Barcode', 'Gambar'])
  })

  it('nilai awal tambah: harga 0, kelola none, aktif, barcodes kosong', () => {
    const v = initialValues(productFields, null)
    expect(v).toMatchObject({ base_price: '0', manage_by: 'none', track_stock: true, is_active: true, barcodes: [], sku: '' })
  })

  it('nilai awal edit membawa barcode dengan sumbernya', () => {
    const v = initialValues(productFields, detail)
    expect(v.barcodes).toEqual([
      { id: 'b1', code: '8990012', unit_id: 'dus', note: '', source: 'local' },
      { id: 'b2', code: 'SAP1', unit_id: 'pcs', note: 'x', source: 'sap' },
    ])
    expect(v.uom_group_id).toBe('grp')
  })

  it('payload: kosong jadi null, gambar tidak ikut, barcodes ikut utuh', () => {
    const v = { ...initialValues(productFields, detail), brand_id: '', barcode: '', sales_unit_id: '' }
    const p = buildPayload(productFields, v, false)
    expect(p).not.toHaveProperty('image')
    expect(p).toMatchObject({ brand_id: null, barcode: null, sales_unit_id: null, base_price: '3500.00', sku: 'SKU1', uom_group_id: 'grp', manage_by: 'batch' })
    expect(p.barcodes).toHaveLength(2)
  })

  it('payload tambah: wajib kosong tetap string kosong agar server menolak dengan pesan', () => {
    const p = buildPayload(productFields, initialValues(productFields, null), true)
    expect(p).toMatchObject({ sku: '', name: '', unit_id: '', product_group_id: '' })
  })

  it('error bersarang barcodes.* menandai tab Satuan & Barcode', () => {
    const t = tabsWithErrors(productFields, { 'barcodes.0.code': 'Sudah dipakai produk lain.', sku: 'Sudah digunakan.' })
    expect([...t].sort()).toEqual(['Satuan & Barcode', 'Umum'])
    expect(tabsWithErrors(productFields, { sku: '' }).size).toBe(0)
  })
})

describe('satuan produk', () => {
  const group = { lines: [{ unit_id: 'pcs' }, { unit_id: 'dus' }, { unit_id: 'pak' }] }
  it('satuan dasar dulu, lalu anggota group tanpa duplikat', () => {
    expect(allowedUnitIds('pcs', group)).toEqual(['pcs', 'dus', 'pak'])
  })
  it('tanpa group hanya satuan dasar; tanpa satuan dasar kosong', () => {
    expect(allowedUnitIds('pcs', null)).toEqual(['pcs'])
    expect(allowedUnitIds('', group)).toEqual([])
  })
  it('baris barcode baru memakai satuan dasar dan lokal', () => {
    expect(newBarcodeRow('pcs')).toEqual({ code: '', unit_id: 'pcs', note: '', source: 'local' })
    expect(barcodeRows({})).toEqual([])
  })
})
