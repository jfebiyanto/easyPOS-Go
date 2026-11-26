import { useEffect, useState } from 'react'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { Trash2 } from 'lucide-react'
import { api, ApiError, type Me } from '../../shared/api'
import { RecordForm, FormField, MoneyInput, inputCls } from '../../shared/ui/controls'
import { ActiveBadge, CrudPage, type CrudConfig } from '../../shared/ui/CrudPage'
import { buildPayload, fieldErrors, formatDecimal, initialValues, type FieldDef, type Row } from '../../shared/ui/crudLogic'
import { ConfirmDialog, useToast } from '../../shared/ui/overlays'

const active: FieldDef = { name: 'is_active', label: 'Aktif', type: 'switch', default: true }
const activeCol = { key: 'is_active', label: 'Status', render: (r: Row) => <ActiveBadge active={r.is_active} /> }
const yesNo = (v: boolean) => (v ? 'Ya' : 'Tidak')

/* ---------- Grup Pajak ---------- */
const taxGroups: CrudConfig = {
  title: 'Grup Pajak', noun: 'grup pajak', path: '/api/tax-groups', permKey: 'admin.tax-groups.index',
  columns: [
    { key: 'code', label: 'Kode' }, { key: 'name', label: 'Nama' },
    { key: 'rate', label: 'Tarif', className: 'text-right', render: (r) => `${formatDecimal(r.rate)}%` },
    { key: 'effective_from', label: 'Berlaku mulai', render: (r) => r.effective_from ?? '—' },
    activeCol,
  ],
  fields: [
    { name: 'code', label: 'Kode', type: 'text', required: true, maxLength: 30 },
    { name: 'name', label: 'Nama', type: 'text', required: true, maxLength: 150 },
    { name: 'rate', label: 'Tarif (%)', type: 'money', required: true, hint: '0 sampai 100, maksimal 2 desimal.' },
    { name: 'effective_from', label: 'Berlaku mulai', type: 'date' },
    active,
  ],
  canDelete: (r) => r.source !== 'sap',
}
export const TaxGroupsPage = ({ me }: { me: Me }) => <CrudPage me={me} cfg={taxGroups} />

/* ---------- Mata Uang (+ denominasi) ---------- */
function Denominations({ rec, canWrite, refresh }: { rec: Row; canWrite: boolean; refresh: () => void }) {
  const toast = useToast()
  const [value, setValue] = useState('')
  const [error, setError] = useState('')
  const [del, setDel] = useState<Row | null>(null)
  const add = useMutation({
    mutationFn: () => api(`/api/currencies/${rec.id}/denominations`, { method: 'POST', body: { value } }),
    onSuccess: () => { setValue(''); setError(''); refresh() },
    onError: (e) => setError(fieldErrors(e).value ?? e.message),
  })
  const remove = useMutation({
    mutationFn: (d: Row) => api(`/api/currencies/${rec.id}/denominations/${d.id}`, { method: 'DELETE' }),
    onSuccess: () => { setDel(null); refresh() },
    onError: (e) => { toast(e.message, 'error'); setDel(null) },
  })
  const dens: Row[] = rec.denominations ?? []
  return (
    <div>
      <h3 className="mb-2 font-medium">Denominasi</h3>
      {dens.length === 0 && <p className="text-sm text-muted">Belum ada denominasi.</p>}
      <ul className="mb-3 flex flex-wrap gap-2">
        {dens.map((d) => (
          <li key={d.id} className="flex items-center gap-1 rounded-full border border-border bg-surface-2 py-1 pl-3 pr-1 text-sm tabular-nums">
            {rec.symbol} {formatDecimal(d.value)}
            {canWrite && <button type="button" onClick={() => setDel(d)} aria-label={`Hapus denominasi ${d.value}`} className="rounded-full p-1.5 text-danger hover:bg-surface"><Trash2 size={14} aria-hidden="true" /></button>}
          </li>
        ))}
      </ul>
      {canWrite && (
        <div className="flex items-start gap-2">
          <div className="flex-1">
            <FormField label="Nominal baru" error={error}>
              {({ id, describedBy, invalid }) => <MoneyInput id={id} aria-describedby={describedBy} aria-invalid={invalid} value={value} onChange={setValue} />}
            </FormField>
          </div>
          <button type="button" disabled={!value || add.isPending} onClick={() => add.mutate()} className="mt-6 rounded-xl border border-border px-4 py-2 text-sm hover:bg-surface-2 disabled:opacity-50">Tambah</button>
        </div>
      )}
      <ConfirmDialog open={!!del} title="Hapus denominasi?" message="Nominal ini akan dihapus dari mata uang." busy={remove.isPending}
        onConfirm={() => del && remove.mutate(del)} onCancel={() => setDel(null)} />
    </div>
  )
}

const currencies: CrudConfig = {
  title: 'Mata Uang', noun: 'mata uang', path: '/api/currencies', permKey: 'admin.currencies.index', searchHint: 'Cari kode',
  columns: [
    { key: 'code', label: 'Kode' }, { key: 'symbol', label: 'Simbol' },
    { key: 'decimal_places', label: 'Desimal', className: 'text-right' }, activeCol,
  ],
  fields: [
    { name: 'code', label: 'Kode', type: 'text', required: true, maxLength: 3, hint: '3 huruf, mis. IDR.' },
    { name: 'symbol', label: 'Simbol', type: 'text', required: true, maxLength: 10 },
    { name: 'decimal_places', label: 'Jumlah desimal', type: 'number', required: true, default: 2, hint: '0 sampai 4.' },
    active,
  ],
  extra: (rec, canWrite, refresh) => <Denominations rec={rec} canWrite={canWrite} refresh={refresh} />,
}
export const CurrenciesPage = ({ me }: { me: Me }) => <CrudPage me={me} cfg={currencies} />

/* ---------- Gudang ---------- */
const warehouses: CrudConfig = {
  title: 'Gudang', noun: 'gudang', path: '/api/warehouses', permKey: 'admin.warehouses.index',
  columns: [
    { key: 'code', label: 'Kode' }, { key: 'name', label: 'Nama' },
    { key: 'is_retail', label: 'Retail', render: (r) => yesNo(r.is_retail) }, activeCol,
  ],
  fields: [
    { name: 'code', label: 'Kode', type: 'text', required: true, maxLength: 30 },
    { name: 'name', label: 'Nama', type: 'text', required: true, maxLength: 150 },
    { name: 'profit_center', label: 'Profit center', type: 'text', maxLength: 50 },
    { name: 'is_retail', label: 'Gudang retail', type: 'switch', default: true },
    { name: 'is_transit', label: 'Gudang transit', type: 'switch' },
    { name: 'transit_warehouse_code', label: 'Kode gudang transit', type: 'text', maxLength: 30 },
    { name: 'cash_account', label: 'Akun kas', type: 'text', maxLength: 25 },
    { name: 'qris_account', label: 'Akun QRIS', type: 'text', maxLength: 25 },
    { name: 'edc_account', label: 'Akun EDC', type: 'text', maxLength: 25 },
    active,
  ],
}
export const WarehousesPage = ({ me }: { me: Me }) => <CrudPage me={me} cfg={warehouses} />

/* ---------- Gerai ---------- */
// Price list dan pelanggan default menyusul di F1.6 / F1.8 (resource-nya belum ada).
const stores: CrudConfig = {
  title: 'Gerai', noun: 'gerai', path: '/api/stores', permKey: 'admin.stores.index',
  columns: [
    { key: 'code', label: 'Kode' }, { key: 'name', label: 'Nama' }, { key: 'pic_name', label: 'PIC' },
    { key: 'warehouse', label: 'Gudang', render: (r) => r.warehouse?.name ?? '—' }, activeCol,
  ],
  fields: [
    { name: 'code', label: 'Kode', type: 'text', required: true, maxLength: 20 },
    { name: 'name', label: 'Nama', type: 'text', required: true, maxLength: 150 },
    { name: 'address', label: 'Alamat', type: 'textarea', maxLength: 500 },
    { name: 'pic_name', label: 'Nama PIC', type: 'text', required: true, maxLength: 150 },
    { name: 'pic_phone', label: 'Telepon PIC', type: 'text', maxLength: 30 },
    { name: 'warehouse_id', label: 'Gudang', type: 'select', ref: { path: '/api/warehouses', label: (r) => `${r.code} · ${r.name}` }, hint: 'Satu gudang hanya untuk satu gerai.' },
    { name: 'currency', label: 'Mata uang', type: 'select', required: true, ref: { path: '/api/currencies', value: 'code', label: (r) => `${r.code} · ${r.symbol}` } },
    { name: 'ppn_enabled', label: 'PPN aktif', type: 'switch' },
    { name: 'ppn_rate', label: 'Tarif PPN (%)', type: 'money', default: '11.00', showWhen: (v) => v.ppn_enabled === true },
    { name: 'opening_float', label: 'Modal awal kas', type: 'money', default: '0.00' },
    active,
  ],
}
export const StoresPage = ({ me }: { me: Me }) => <CrudPage me={me} cfg={stores} />

/* ---------- Pengguna (+ reset password) ---------- */
function ResetPassword({ rec }: { rec: Row }) {
  const toast = useToast()
  const [pw, setPw] = useState('')
  const [conf, setConf] = useState('')
  const [errors, setErrors] = useState<Record<string, string>>({})
  const reset = useMutation({
    mutationFn: () => api(`/api/users/${rec.id}/reset-password`, { method: 'POST', body: { password: pw, password_confirmation: conf } }),
    onSuccess: () => { toast('Password direset. Semua sesi pengguna ini dicabut.'); setPw(''); setConf(''); setErrors({}) },
    onError: (e) => { setErrors(fieldErrors(e)); if (!(e instanceof ApiError && e.status === 422)) toast(e.message, 'error') },
  })
  return (
    <form onSubmit={(e) => { e.preventDefault(); setErrors({}); reset.mutate() }}>
      <h3 className="mb-2 font-medium">Reset password</h3>
      <FormField label="Password baru" error={errors.password} hint="Minimal 10 karakter, berisi huruf dan angka.">
        {({ id, describedBy, invalid }) => <input id={id} type="password" autoComplete="new-password" aria-describedby={describedBy} aria-invalid={invalid} value={pw} onChange={(e) => setPw(e.target.value)} className={inputCls} />}
      </FormField>
      <FormField label="Konfirmasi password" error={errors.password_confirmation}>
        {({ id, describedBy, invalid }) => <input id={id} type="password" autoComplete="new-password" aria-describedby={describedBy} aria-invalid={invalid} value={conf} onChange={(e) => setConf(e.target.value)} className={inputCls} />}
      </FormField>
      <button type="submit" disabled={!pw || reset.isPending} className="rounded-xl border border-border px-4 py-2 text-sm hover:bg-surface-2 disabled:opacity-50">
        {reset.isPending ? 'Memproses…' : 'Reset password'}
      </button>
    </form>
  )
}

const roleOpts = [
  { value: 'owner', label: 'Owner' }, { value: 'store_manager', label: 'Store Manager' }, { value: 'cashier', label: 'Kasir' },
]
const roleLabel = Object.fromEntries(roleOpts.map((o) => [o.value, o.label]))

const users: CrudConfig = {
  title: 'Pengguna', noun: 'pengguna', path: '/api/users', permKey: 'admin.users.index', searchHint: 'Cari nama, username, email',
  columns: [
    { key: 'name', label: 'Nama' }, { key: 'username', label: 'Username' },
    { key: 'role', label: 'Peran', render: (r) => roleLabel[r.role] ?? r.role },
    { key: 'store', label: 'Gerai', render: (r) => r.store?.name ?? '—' }, activeCol,
  ],
  fields: [
    { name: 'name', label: 'Nama', type: 'text', required: true, maxLength: 150 },
    { name: 'username', label: 'Username', type: 'text', required: true, maxLength: 60, hint: 'Huruf, angka, strip, dan underscore.' },
    { name: 'email', label: 'Email', type: 'text', maxLength: 150 },
    { name: 'phone', label: 'Telepon', type: 'text', maxLength: 30 },
    { name: 'role', label: 'Peran', type: 'select', required: true, options: roleOpts },
    { name: 'store_id', label: 'Gerai', type: 'select', ref: { path: '/api/stores', label: (r) => `${r.code} · ${r.name}` }, showWhen: (v) => v.role !== 'owner', required: true },
    { name: 'password', label: 'Password', type: 'password', required: true, createOnly: true, hint: 'Minimal 10 karakter, berisi huruf dan angka.' },
    active,
  ],
  extra: (rec, canWrite) => (canWrite ? <ResetPassword rec={rec} /> : null),
}
export const UsersPage = ({ me }: { me: Me }) => <CrudPage me={me} cfg={users} />

/* ---------- Profil Perusahaan (singleton) ---------- */
const companyFields: FieldDef[] = [
  { name: 'name', label: 'Nama perusahaan', type: 'text', required: true, maxLength: 150 },
  { name: 'address', label: 'Alamat', type: 'textarea', maxLength: 500 },
  { name: 'npwp_number', label: 'NPWP', type: 'npwp', hint: '16 digit; disimpan tanpa titik.' },
  { name: 'phone', label: 'Telepon', type: 'text', maxLength: 30 },
  { name: 'email', label: 'Email', type: 'text', maxLength: 150 },
  { name: 'base_currency_code', label: 'Mata uang dasar', type: 'select', required: true, ref: { path: '/api/currencies', value: 'code', label: (r) => `${r.code} · ${r.symbol}` } },
  { name: 'timezone', label: 'Zona waktu', type: 'text', required: true, hint: 'Nama IANA, mis. Asia/Jakarta.' },
  { name: 'ppn_enabled', label: 'PPN aktif', type: 'switch' },
  { name: 'tax_group_id', label: 'Grup pajak', type: 'select', ref: { path: '/api/tax-groups', label: (r) => `${r.code} · ${r.name}` }, showWhen: (v) => v.ppn_enabled === true, required: true },
  { name: 'ppn_inclusive', label: 'Harga sudah termasuk PPN', type: 'switch' },
  { name: 'allow_negative_stock', label: 'Izinkan stok negatif', type: 'switch' },
  { name: 'return_window_days', label: 'Batas retur (hari)', type: 'number', default: 30, hint: '0 sampai 3650.' },
  { name: 'return_require_receipt', label: 'Retur wajib struk', type: 'switch', default: true },
]

export function CompanyPage({ me }: { me: Me }) {
  const qc = useQueryClient()
  const toast = useToast()
  const allowed = me.permissions['admin.company.edit']?.visible
  const { data, error } = useQuery({ queryKey: ['company'], queryFn: () => api<{ data: Row }>('/api/company'), enabled: !!allowed })
  const [values, setValues] = useState(() => initialValues(companyFields))
  const [errors, setErrors] = useState<Record<string, string>>({})
  useEffect(() => { if (data) setValues(initialValues(companyFields, data.data)) }, [data])

  const save = useMutation({
    mutationFn: () => api<{ data: Row }>('/api/company', { method: 'PUT', body: buildPayload(companyFields, values, false) }),
    onSuccess: (r) => { qc.setQueryData(['company'], r); qc.invalidateQueries({ queryKey: ['me'] }); toast('Profil perusahaan disimpan.') },
    onError: (e) => { setErrors(fieldErrors(e)); if (!(e instanceof ApiError && e.status === 422)) toast(e.message, 'error') },
  })

  if (!allowed) return <p role="alert" className="text-danger">Hanya Owner yang dapat mengubah profil perusahaan.</p>
  if (error) return <p role="alert" className="text-danger">{error.message}</p>
  if (!data) return <p className="text-muted">Memuat…</p>
  return (
    <form onSubmit={(e) => { e.preventDefault(); setErrors({}); save.mutate() }} className="max-w-4xl rounded-[14px] border border-border bg-surface p-4 md:p-6">
      <h1 className="mb-4 text-xl font-semibold">Profil Perusahaan</h1>
      <RecordForm columns={2} fields={companyFields} values={values} errors={errors} isCreate={false}
        onChange={(n, v) => { setValues({ ...values, [n]: v }); if (errors[n]) setErrors({ ...errors, [n]: '' }) }} />
      <button type="submit" disabled={save.isPending} className="rounded-xl bg-primary px-5 py-2.5 text-sm font-medium text-on-primary hover:bg-primary-hover disabled:opacity-60">
        {save.isPending ? 'Menyimpan…' : 'Simpan'}
      </button>
    </form>
  )
}
