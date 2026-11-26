import { ApiError } from '../api'

export type Opt = { value: string; label: string }
export type Values = Record<string, unknown>
// eslint-disable-next-line @typescript-eslint/no-explicit-any
export type Row = Record<string, any>

export type FieldDef = {
  name: string
  label: string
  type: 'text' | 'textarea' | 'number' | 'money' | 'switch' | 'select' | 'password' | 'date'
  required?: boolean
  hint?: string
  default?: unknown
  options?: Opt[]
  /** Opsi dari API daftar (per_page=100). value default = id. */
  ref?: { path: string; value?: string; label: (r: Row) => string }
  createOnly?: boolean
  showWhen?: (v: Values) => boolean
  maxLength?: number
}

const emptyDefault = (f: FieldDef): unknown => (f.type === 'switch' ? false : '')

/** Nilai awal formulir: dari record (edit) atau default field (baru). */
export function initialValues(fields: FieldDef[], rec?: Row | null): Values {
  const v: Values = {}
  for (const f of fields) {
    if (rec) v[f.name] = f.type === 'password' ? '' : (rec[f.name] ?? emptyDefault(f))
    else v[f.name] = f.default ?? emptyDefault(f)
  }
  return v
}

/** Payload ke API: string kosong -> null (kecuali wajib), number -> Number, field tersembunyi/createOnly/password kosong dibuang. */
export function buildPayload(fields: FieldDef[], values: Values, isCreate: boolean): Row {
  const out: Row = {}
  for (const f of fields) {
    if (f.showWhen && !f.showWhen(values)) continue
    if (f.createOnly && !isCreate) continue
    const v = values[f.name]
    if (f.type === 'password' && (v === '' || v == null)) continue
    if (f.type === 'switch') out[f.name] = !!v
    else if (v === '' || v == null) out[f.name] = f.required ? '' : null
    else if (f.type === 'number') out[f.name] = Number(v)
    else out[f.name] = v
  }
  return out
}

/** Pesan pertama per field dari respons 422. */
export function fieldErrors(err: unknown): Record<string, string> {
  if (!(err instanceof ApiError)) return {}
  return Object.fromEntries(Object.entries(err.errors).map(([k, v]) => [k, v[0] ?? '']))
}

/** Hanya digit dan satu titik desimal; selalu string (uang tidak pernah float). */
export function cleanMoney(raw: string): string {
  const s = raw.replace(/[^\d.]/g, '')
  const i = s.indexOf('.')
  return i < 0 ? s : s.slice(0, i + 1) + s.slice(i + 1).replace(/\./g, '')
}

/** Tampilan angka desimal bergaya Indonesia dari string server ("12500.00" -> "12.500,00"). */
export function formatDecimal(s: string | null | undefined): string {
  if (s == null || s === '') return '—'
  const [int, frac] = s.split('.')
  const grouped = int.replace(/\B(?=(\d{3})+(?!\d))/g, '.')
  return frac ? `${grouped},${frac}` : grouped
}
