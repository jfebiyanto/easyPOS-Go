import { describe, expect, it } from 'vitest'
import { ApiError } from '../api'
import { buildPayload, cleanMoney, conversionText, fieldErrors, formatDecimal, formatNpwp, initialValues, nextSort, npwpDigits, npwpError, type FieldDef } from './crudLogic'

const fields: FieldDef[] = [
  { name: 'code', label: 'Kode', type: 'text', required: true },
  { name: 'note', label: 'Catatan', type: 'text' },
  { name: 'rate', label: 'Tarif', type: 'money' },
  { name: 'days', label: 'Hari', type: 'number' },
  { name: 'is_active', label: 'Aktif', type: 'switch', default: true },
  { name: 'password', label: 'Password', type: 'password', createOnly: false },
  { name: 'pin', label: 'PIN', type: 'text', createOnly: true },
  { name: 'tax', label: 'Pajak', type: 'select', showWhen: (v) => v.is_active === true },
]

describe('initialValues', () => {
  it('baru: default field; edit: dari record, password dikosongkan', () => {
    expect(initialValues(fields).is_active).toBe(true)
    const v = initialValues(fields, { code: 'A', note: null, is_active: false, password: 'x' })
    expect(v).toMatchObject({ code: 'A', note: '', is_active: false, password: '' })
  })
})

describe('buildPayload', () => {
  it('kosong -> null, kecuali wajib; number -> Number; money tetap string', () => {
    const p = buildPayload(fields, { code: '', note: '', rate: '11.50', days: '30', is_active: true, password: '', pin: '', tax: '' }, true)
    expect(p).toMatchObject({ code: '', note: null, rate: '11.50', days: 30, is_active: true, pin: null, tax: null })
    expect('password' in p).toBe(false)
  })
  it('edit: createOnly dibuang; password terisi dikirim', () => {
    const p = buildPayload(fields, { code: 'A', password: 'rahasia', pin: '1', is_active: true }, false)
    expect('pin' in p).toBe(false)
    expect(p.password).toBe('rahasia')
  })
  it('field tersembunyi (showWhen) tidak dikirim', () => {
    expect('tax' in buildPayload(fields, { code: 'A', is_active: false, tax: 'x' }, true)).toBe(false)
  })
})

describe('fieldErrors', () => {
  it('mengambil pesan pertama per field; error non-422 = kosong', () => {
    expect(fieldErrors(new ApiError(422, 'x', { code: ['Wajib.', 'Lain'], rate: ['Salah.'] }))).toEqual({ code: 'Wajib.', rate: 'Salah.' })
    expect(fieldErrors(new Error('x'))).toEqual({})
  })
})

describe('angka', () => {
  it('cleanMoney: hanya digit dan satu titik', () => {
    expect(cleanMoney('12a3.4.5')).toBe('123.45')
    expect(cleanMoney('Rp 1.000')).toBe('1.000')
    expect(cleanMoney('')).toBe('')
  })
  it('formatDecimal: pemisah ribuan titik, desimal koma', () => {
    expect(formatDecimal('12500.00')).toBe('12.500,00')
    expect(formatDecimal('1234567')).toBe('1.234.567')
    expect(formatDecimal(null)).toBe('—')
  })
})

describe('field custom', () => {
  const lines: FieldDef = { name: 'lines', label: 'Baris', type: 'custom', default: [], fromRecord: (r) => (r.lines as unknown[]).slice(1) }
  it('initialValues memakai fromRecord pada edit dan default pada baru', () => {
    expect(initialValues([lines]).lines).toEqual([])
    expect(initialValues([lines], { lines: ['dasar', 'dus'] }).lines).toEqual(['dus'])
  })
  it('buildPayload membawa nilai custom apa adanya', () => {
    const v = [{ unit_id: 'a', alt_qty: '1', base_qty: '12' }]
    expect(buildPayload([lines], { lines: v }, false).lines).toBe(v)
  })
})

describe('conversionText', () => {
  it('membuang nol di belakang koma dan menandai yang belum dipilih', () => {
    expect(conversionText('1.000', 'Dus', '12.000', 'Pcs')).toBe('1 Dus = 12 Pcs')
    expect(conversionText('2', 'Pak', '5', 'Pcs')).toBe('2 Pak = 5 Pcs')
    expect(conversionText('100.000', 'Dus', '2.500', 'Pcs')).toBe('100 Dus = 2.5 Pcs')
    expect(conversionText('', '', '', '')).toBe('0 ? = 0 ?')
  })
})

describe('nextSort', () => {
  it('kolom baru asc, kolom sama bergantian', () => {
    expect(nextSort({ key: 'code', dir: 'asc' }, 'name')).toEqual({ key: 'name', dir: 'asc' })
    expect(nextSort({ key: 'code', dir: 'asc' }, 'code')).toEqual({ key: 'code', dir: 'desc' })
    expect(nextSort({ key: 'code', dir: 'desc' }, 'code')).toEqual({ key: 'code', dir: 'asc' })
  })
})

describe('NPWP', () => {
  it('format xx.xxxx.xxxxxx.xxxx, digit saja yang disimpan', () => {
    expect(formatNpwp('0123456789012345')).toBe('01.2345.678901.2345')
    expect(formatNpwp('01.2345.678901.2345')).toBe('01.2345.678901.2345')
    expect(formatNpwp('01234')).toBe('01.234')
    expect(formatNpwp('')).toBe('')
    expect(npwpDigits('01.2345.678901.2345 99')).toBe('0123456789012345')
  })
  it('validasi lost focus: terisi harus 16 digit, kosong lolos', () => {
    expect(npwpError('01.234')).toBe('NPWP harus 16 digit angka.')
    expect(npwpError('01.2345.678901.2345')).toBe('')
    expect(npwpError('')).toBe('')
  })
  it('form: tampil terformat dari record, payload 16 digit atau null', () => {
    const f: FieldDef[] = [{ name: 'npwp', label: 'NPWP', type: 'npwp' }]
    const v = initialValues(f, { npwp: '0123456789012345' })
    expect(v.npwp).toBe('01.2345.678901.2345')
    expect(buildPayload(f, v, false)).toEqual({ npwp: '0123456789012345' })
    expect(buildPayload(f, { npwp: '' }, false)).toEqual({ npwp: null })
  })
})
