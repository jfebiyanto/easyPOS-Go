import { describe, expect, it } from 'vitest'
import { ApiError } from '../api'
import { buildPayload, cleanMoney, fieldErrors, formatDecimal, initialValues, type FieldDef } from './crudLogic'

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
