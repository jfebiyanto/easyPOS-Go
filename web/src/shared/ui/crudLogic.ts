import type { ReactNode } from 'react'
import { ApiError } from '../api'

export type Opt = { value: string; label: string }
export type Values = Record<string, unknown>
// eslint-disable-next-line @typescript-eslint/no-explicit-any
export type Row = Record<string, any>

export type CustomCtx = {
  values: Values
  set: (v: unknown) => void
  errors: Record<string, string>
  disabled?: boolean
  /** Ubah field lain (mis. satuan dasar mengikuti UoM group). */
  setField: (name: string, v: unknown) => void
  /** Record dari server saat edit (null saat tambah). */
  record: Row | null
  /** Muat ulang daftar + detail setelah aksi di luar formulir (mis. unggah gambar). */
  refresh: () => void
}

export type FieldDef = {
  name: string
  label: string
  type: 'text' | 'textarea' | 'number' | 'money' | 'switch' | 'select' | 'password' | 'date' | 'npwp' | 'custom'
  /** Hanya type 'custom': render sendiri; nilai dibawa apa adanya ke payload. */
  render?: (c: CustomCtx) => ReactNode
  /** Nilai awal field custom dari record (edit). */
  fromRecord?: (rec: Row) => unknown
  required?: boolean
  hint?: string
  default?: unknown
  options?: Opt[]
  /** Opsi dari API daftar (per_page=100). value default = id. */
  ref?: { path: string; value?: string; label: (r: Row) => string }
  createOnly?: boolean
  showWhen?: (v: Values) => boolean
  /** Nonaktif (read-only) secara kondisional. */
  disabledWhen?: (v: Values) => boolean
  /** Nama tab di formulir bertab; field tanpa tab ikut tab pertama. */
  tab?: string
  /** Tidak ikut payload (mis. unggah gambar yang punya endpoint sendiri). */
  noPayload?: boolean
  maxLength?: number
}

const emptyDefault = (f: FieldDef): unknown => (f.type === 'switch' ? false : '')

/** Nilai awal formulir: dari record (edit) atau default field (baru). */
export function initialValues(fields: FieldDef[], rec?: Row | null): Values {
  const v: Values = {}
  for (const f of fields) {
    if (rec && f.fromRecord) v[f.name] = f.fromRecord(rec)
    else if (rec) v[f.name] = f.type === 'password' ? '' : f.type === 'npwp' ? formatNpwp(String(rec[f.name] ?? '')) : (rec[f.name] ?? emptyDefault(f))
    else v[f.name] = f.default ?? emptyDefault(f)
  }
  return v
}

/** Hanya digit NPWP (maks 16): inilah yang disimpan di database. */
export const npwpDigits = (s: string): string => s.replace(/\D/g, '').slice(0, 16)

/** Tampilan NPWP sesuai pencatatan pajak: xx.xxxx.xxxxxx.xxxx (parsial bila digit belum 16). */
export function formatNpwp(s: string): string {
  const d = npwpDigits(s)
  return [d.slice(0, 2), d.slice(2, 6), d.slice(6, 12), d.slice(12, 16)].filter(Boolean).join('.')
}

/** Pesan salah bila NPWP terisi tapi bukan 16 digit; kosong dianggap valid (wajib-tidaknya urusan field). */
export const npwpError = (s: string): string => (npwpDigits(s) && npwpDigits(s).length !== 16 ? 'NPWP harus 16 digit angka.' : '')

/** Payload ke API: string kosong -> null (kecuali wajib), number -> Number, field tersembunyi/createOnly/password kosong dibuang. */
export function buildPayload(fields: FieldDef[], values: Values, isCreate: boolean): Row {
  const out: Row = {}
  for (const f of fields) {
    if (f.showWhen && !f.showWhen(values)) continue
    if (f.createOnly && !isCreate) continue
    if (f.noPayload) continue
    const v = values[f.name]
    if (f.type === 'password' && (v === '' || v == null)) continue
    if (f.type === 'custom') out[f.name] = v === '' || v == null ? (f.required ? '' : null) : v
    else if (f.type === 'switch') out[f.name] = !!v
    else if (v === '' || v == null) out[f.name] = f.required ? '' : null
    else if (f.type === 'number') out[f.name] = Number(v)
    else if (f.type === 'npwp') out[f.name] = npwpDigits(String(v)) || (f.required ? '' : null)
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

/** Kalimat konversi UoM: "1 Dus = 12 Pcs" (nol di belakang koma dibuang). */
export function conversionText(alt: string, unitCode: string, base: string, baseCode: string): string {
  const trim = (s: string) => (s.includes('.') ? s.replace(/\.?0+$/, '') : s)
  return `${trim(alt || '0')} ${unitCode || '?'} = ${trim(base || '0')} ${baseCode || '?'}`
}

export type Sort = { key: string; dir: 'asc' | 'desc' }

/** Klik header: kolom baru → asc; kolom yang sama → asc↔desc. */
export function nextSort(cur: Sort, key: string): Sort {
  return cur.key === key ? { key, dir: cur.dir === 'asc' ? 'desc' : 'asc' } : { key, dir: 'asc' }
}

/** Urutan nama tab (menurut kemunculan pertama); field tanpa tab ikut tab pertama. */
export function tabNames(fields: FieldDef[]): string[] {
  const out: string[] = []
  for (const f of fields) if (f.tab && !out.includes(f.tab)) out.push(f.tab)
  return out
}

/** Tab yang berisi error (kunci error bisa bersarang, mis. "barcodes.0.code" milik field "barcodes"). */
export function tabsWithErrors(fields: FieldDef[], errors: Record<string, string>): Set<string> {
  const tabs = tabNames(fields)
  const out = new Set<string>()
  for (const [k, msg] of Object.entries(errors)) {
    if (!msg) continue
    const f = fields.find((x) => k === x.name || k.startsWith(x.name + '.'))
    if (f) out.add(f.tab ?? tabs[0])
  }
  return out
}
