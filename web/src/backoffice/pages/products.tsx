import { useRef, useState } from 'react'
import { useQuery } from '@tanstack/react-query'
import { ImagePlus, Package, Plus, Trash2 } from 'lucide-react'
import { api, ApiError, type Me } from '../../shared/api'
import { FormField, Select, useOptions } from '../../shared/ui/controls'
import { ActiveBadge, CrudPage, type CrudConfig } from '../../shared/ui/CrudPage'
import { formatDecimal, type CustomCtx, type FieldDef, type Opt, type Row } from '../../shared/ui/crudLogic'
import { useToast } from '../../shared/ui/overlays'
import { allowedUnitIds, barcodeRows, newBarcodeRow, type BarcodeRow } from './productLogic'

const codeName = (r: Row) => `${r.code} · ${r.name}`
const unitRef = { path: '/api/units', label: codeName }
const groupRef = { path: '/api/uom-groups', label: codeName }
const manageBy: Opt[] = [{ value: 'none', label: 'Tidak ada' }, { value: 'batch', label: 'Batch' }, { value: 'serial', label: 'Serial' }]

const TAB_GENERAL = 'Umum', TAB_UNITS = 'Satuan & Barcode', TAB_IMAGE = 'Gambar'

/** Satuan yang boleh dipilih: satuan dasar + anggota UoM group terpilih (detail group dimuat bila ada). */
function useAllowedUnits(values: CustomCtx['values']): Opt[] {
  const units = useOptions(unitRef)
  const groupId = String(values.uom_group_id ?? '')
  const group = useQuery({ queryKey: ['uom-group', groupId], queryFn: () => api<{ data: Row }>(`/api/uom-groups/${groupId}`), enabled: !!groupId, staleTime: 30_000 })
  const ids = allowedUnitIds(String(values.unit_id ?? ''), groupId ? group.data?.data : null)
  return ids.map((id) => units.find((u) => u.value === id)).filter((o): o is Opt => !!o)
}

function GroupField({ values, set, setField, errors, disabled }: CustomCtx) {
  const opts = useOptions(groupRef)
  async function change(id: string) {
    set(id)
    setField('sales_unit_id', '')
    if (id) { // satuan dasar produk mengikuti satuan dasar group
      const g = await api<{ data: Row }>(`/api/uom-groups/${id}`).catch(() => null)
      if (g) setField('unit_id', g.data.base_unit_id)
    }
  }
  return (
    <FormField label="UoM group" hint="Opsional. Bila dipilih, satuan dasar produk mengikuti satuan dasar group." error={errors.uom_group_id}>
      {({ id, describedBy, invalid }) => <Select id={id} aria-describedby={describedBy} aria-invalid={invalid} disabled={disabled} options={opts} value={String(values.uom_group_id ?? '')} onChange={(v) => void change(v)} />}
    </FormField>
  )
}

function SalesUnitField({ values, set, errors, disabled }: CustomCtx) {
  const opts = useAllowedUnits(values)
  return (
    <FormField label="Satuan jual default" hint="Satuan yang terisi otomatis di kasir. Kosong = satuan dasar." error={errors.sales_unit_id}>
      {({ id, describedBy, invalid }) => <Select id={id} aria-describedby={describedBy} aria-invalid={invalid} disabled={disabled} options={opts} value={String(values.sales_unit_id ?? '')} onChange={set} />}
    </FormField>
  )
}

function BarcodesEditor({ values, set, errors, disabled }: CustomCtx) {
  const opts = useAllowedUnits(values)
  const rows = (values.barcodes as BarcodeRow[]) ?? []
  const patch = (i: number, p: Partial<BarcodeRow>) => set(rows.map((r, j) => (j === i ? { ...r, ...p } : r)))
  const baseUnit = String(values.unit_id ?? '')
  const field = 'w-full min-w-0 rounded-xl border border-border bg-surface px-3 py-2 text-sm text-text disabled:opacity-60 aria-[invalid=true]:border-danger'
  return (
    <fieldset className="mb-4" disabled={disabled}>
      <legend className="mb-1 text-sm font-medium">Barcode per satuan</legend>
      <p className="mb-2 text-xs text-muted">Barcode alternatif (UPC) untuk satuan dasar atau satuan lain di UoM group. Kode harus unik di seluruh produk.</p>
      {errors.barcodes && <p role="alert" className="mb-2 text-xs text-danger">{errors.barcodes}</p>}
      {rows.map((r, i) => {
        const e = (f: string) => errors[`barcodes.${i}.${f}`]
        const sap = r.source === 'sap'
        return (
          <div key={r.id ?? `n${i}`} className="mb-3 rounded-xl border border-border bg-surface-2 p-3">
            <div className="grid grid-cols-[1fr_9rem_auto] items-start gap-2">
              <input aria-label={`Kode barcode baris ${i + 1}`} aria-invalid={!!e('code')} disabled={sap} maxLength={60} value={r.code}
                onChange={(ev) => patch(i, { code: ev.target.value })} placeholder="Kode barcode" className={field} />
              <Select aria-label={`Satuan barcode baris ${i + 1}`} aria-invalid={!!e('unit_id')} disabled={sap} options={opts} value={r.unit_id} onChange={(v) => patch(i, { unit_id: v })} />
              {sap
                ? <span className="px-2 py-2 text-xs text-muted">dari SAP</span>
                : <button type="button" onClick={() => set(rows.filter((_, j) => j !== i))} aria-label={`Hapus barcode baris ${i + 1}`} className="rounded-lg p-2 text-danger hover:bg-surface"><Trash2 size={16} aria-hidden="true" /></button>}
            </div>
            <input aria-label={`Catatan baris ${i + 1}`} maxLength={255} disabled={sap} value={r.note} onChange={(ev) => patch(i, { note: ev.target.value })}
              placeholder="Catatan (opsional)" className={`mt-2 ${field}`} />
            {(e('code') || e('unit_id') || e('id') || e('note')) && <p role="alert" className="mt-1 text-xs text-danger">{e('code') || e('unit_id') || e('id') || e('note')}</p>}
          </div>
        )
      })}
      {!disabled && (
        <button type="button" disabled={!baseUnit} onClick={() => set([...rows, newBarcodeRow(baseUnit)])}
          className="flex items-center gap-2 rounded-xl border border-border px-3 py-2 text-sm hover:bg-surface-2 disabled:opacity-50">
          <Plus size={16} aria-hidden="true" /> Tambah barcode
        </button>
      )}
      {!baseUnit && <p className="mt-1 text-xs text-muted">Pilih satuan dasar dulu.</p>}
    </fieldset>
  )
}

function ImageField({ record, disabled, refresh }: CustomCtx) {
  const toast = useToast()
  const input = useRef<HTMLInputElement>(null)
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState('')
  if (!record) return <p className="text-sm text-muted">Simpan produk dulu, lalu unggah gambarnya di sini.</p>
  const id = record.id
  async function upload(file: File) {
    setBusy(true); setError('')
    const body = new FormData()
    body.append('image', file)
    try { await api(`/api/products/${id}/image`, { method: 'POST', body }); toast('Gambar diunggah.'); refresh() }
    catch (e) { setError(e instanceof ApiError ? (e.errors.image?.[0] ?? e.message) : 'Gagal mengunggah.') }
    finally { setBusy(false); if (input.current) input.current.value = '' }
  }
  async function remove() {
    setBusy(true); setError('')
    try { await api(`/api/products/${id}/image`, { method: 'DELETE' }); toast('Gambar dihapus.'); refresh() }
    catch (e) { setError(e instanceof Error ? e.message : 'Gagal menghapus.') }
    finally { setBusy(false) }
  }
  return (
    <div>
      <div className="flex size-48 items-center justify-center overflow-hidden rounded-2xl border border-border bg-surface-2">
        {record.image_url ? <img src={record.image_url} alt={`Gambar ${record.name}`} className="size-full object-cover" /> : <Package size={48} aria-hidden="true" className="text-muted" />}
      </div>
      <p className="mt-2 text-xs text-muted">JPG, PNG, atau WebP, maksimal 2 MB.</p>
      {error && <p role="alert" className="mt-2 text-sm text-danger">{error}</p>}
      {!disabled && (
        <div className="mt-3 flex gap-2">
          <input ref={input} type="file" accept="image/jpeg,image/png,image/webp" className="sr-only" aria-label="Pilih file gambar"
            onChange={(e) => { const f = e.target.files?.[0]; if (f) void upload(f) }} />
          <button type="button" disabled={busy} onClick={() => input.current?.click()} className="flex items-center gap-2 rounded-xl border border-border px-3 py-2 text-sm hover:bg-surface-2 disabled:opacity-60">
            <ImagePlus size={16} aria-hidden="true" /> {record.image_url ? 'Ganti gambar' : 'Unggah gambar'}
          </button>
          {record.image_url && (
            <button type="button" disabled={busy} onClick={() => void remove()} className="rounded-xl border border-border px-3 py-2 text-sm text-danger hover:bg-surface-2 disabled:opacity-60">Hapus gambar</button>
          )}
        </div>
      )}
    </div>
  )
}

export const productFields: FieldDef[] = [
  { name: 'product_group_id', label: 'Grup produk', type: 'select', required: true, ref: { path: '/api/product-groups', label: codeName }, tab: TAB_GENERAL },
  { name: 'brand_id', label: 'Merek', type: 'select', ref: { path: '/api/brands', label: codeName }, tab: TAB_GENERAL },
  { name: 'sku', label: 'SKU', type: 'text', required: true, maxLength: 40, tab: TAB_GENERAL },
  { name: 'name', label: 'Nama', type: 'text', required: true, maxLength: 150, tab: TAB_GENERAL },
  { name: 'barcode', label: 'Barcode utama', type: 'text', maxLength: 60, hint: 'Barcode satuan dasar.', tab: TAB_GENERAL },
  { name: 'description', label: 'Deskripsi', type: 'textarea', maxLength: 500, tab: TAB_GENERAL },
  { name: 'base_price', label: 'Harga dasar (per satuan dasar)', type: 'money', required: true, default: '0', tab: TAB_GENERAL },
  { name: 'manage_by', label: 'Kelola per', type: 'select', required: true, default: 'none', options: manageBy, hint: 'Tidak bisa diubah bila produk sudah punya batch, serial, atau stok.', tab: TAB_GENERAL },
  { name: 'track_stock', label: 'Lacak stok', type: 'switch', default: true, tab: TAB_GENERAL },
  { name: 'is_active', label: 'Aktif', type: 'switch', default: true, tab: TAB_GENERAL },

  { name: 'uom_group_id', label: 'UoM group', type: 'custom', tab: TAB_UNITS, render: (c) => <GroupField {...c} /> },
  { name: 'unit_id', label: 'Satuan dasar', type: 'select', required: true, ref: unitRef, tab: TAB_UNITS, disabledWhen: (v) => !!v.uom_group_id, hint: 'Stok selalu dihitung dalam satuan ini.' },
  { name: 'sales_unit_id', label: 'Satuan jual default', type: 'custom', tab: TAB_UNITS, render: (c) => <SalesUnitField {...c} /> },
  { name: 'barcodes', label: 'Barcode per satuan', type: 'custom', default: [], tab: TAB_UNITS, fromRecord: barcodeRows, render: (c) => <BarcodesEditor {...c} /> },

  { name: 'image', label: 'Gambar', type: 'custom', noPayload: true, tab: TAB_IMAGE, render: (c) => <ImageField {...c} /> },
]

const products: CrudConfig = {
  title: 'Produk', noun: 'produk', path: '/api/products', permKey: 'admin.products.index', wide: true,
  searchHint: 'Cari SKU, nama, atau barcode',
  filters: [
    { name: 'product_group_id', all: 'Semua grup', ref: { path: '/api/product-groups', label: (r) => r.name } },
    { name: 'brand_id', all: 'Semua merek', ref: { path: '/api/brands', label: (r) => r.name } },
    { name: 'manage_by', all: 'Semua jenis', options: manageBy },
  ],
  columns: [
    {
      key: 'image_url', label: 'Gambar', render: (r) => (
        <span className="flex size-10 items-center justify-center overflow-hidden rounded-lg border border-border bg-surface-2">
          {r.image_url ? <img src={r.image_url} alt="" className="size-full object-cover" /> : <Package size={18} aria-hidden="true" className="text-muted" />}
        </span>
      ),
    },
    { key: 'sku', label: 'SKU' },
    {
      key: 'name', label: 'Nama', render: (r) => (
        <span>
          {r.name}
          {r.manage_by === 'batch' && <span className="ml-2 rounded-full bg-primary-soft px-2 py-0.5 text-xs font-medium text-primary-text">Batch</span>}
          {r.manage_by === 'serial' && <span className="ml-2 rounded-full bg-warn-soft px-2 py-0.5 text-xs font-medium text-warn">Serial</span>}
        </span>
      ),
    },
    { key: 'product_group', label: 'Grup', render: (r) => r.product_group?.name ?? '—' },
    { key: 'unit', label: 'Satuan', render: (r) => r.unit?.code ?? '—' },
    { key: 'base_price', label: 'Harga', className: 'text-right tabular-nums', render: (r) => formatDecimal(r.base_price) },
    { key: 'is_active', label: 'Status', render: (r) => <ActiveBadge active={r.is_active} /> },
  ],
  fields: productFields,
}
export const ProductsPage = ({ me }: { me: Me }) => <CrudPage me={me} cfg={products} />
