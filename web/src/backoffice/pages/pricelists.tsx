import { useState } from 'react'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { Check, Pencil, Plus, Trash2, X } from 'lucide-react'
import { api, ApiError, type Me } from '../../shared/api'
import { MoneyInput, inputCls, useOptions } from '../../shared/ui/controls'
import { ActiveBadge, CrudPage, type CrudConfig } from '../../shared/ui/CrudPage'
import { fieldErrors, formatDecimal, type FieldDef, type Opt, type Row } from '../../shared/ui/crudLogic'
import { ConfirmDialog, useToast } from '../../shared/ui/overlays'
import { allowedUnitIds } from './productLogic'
import { draftFromItem, emptyDraft, itemPayload, itemUnitCode, minQtyError, perBaseText, validityText, type ItemDraft } from './priceListLogic'

const codeName = (r: Row) => `${r.code} · ${r.name}`
const unitRef = { path: '/api/units', label: codeName }
const cell = 'w-full min-w-0 rounded-xl border border-border bg-surface px-2 py-1.5 text-sm text-text disabled:opacity-60 aria-[invalid=true]:border-danger'

export const priceListFields: FieldDef[] = [
  { name: 'code', label: 'Kode', type: 'text', required: true, maxLength: 30 },
  { name: 'name', label: 'Nama', type: 'text', required: true, maxLength: 150 },
  { name: 'store_id', label: 'Gerai', type: 'select', ref: { path: '/api/stores', label: codeName }, hint: 'Kosong = berlaku untuk semua gerai.' },
  { name: 'effective_from', label: 'Berlaku mulai', type: 'date' },
  { name: 'effective_until', label: 'Berlaku sampai', type: 'date' },
  { name: 'is_default', label: 'Price list default', type: 'switch', hint: 'Hanya satu per perusahaan; yang lain otomatis dilepas.' },
  { name: 'is_active', label: 'Aktif', type: 'switch', default: true },
]

/** Satuan yang boleh dipilih untuk sebuah produk: satuan dasar + anggota UoM group-nya. */
function useProductUnits(product: Row | null): Opt[] {
  const units = useOptions(unitRef)
  const groupId = product?.uom_group_id ?? ''
  const group = useQuery({ queryKey: ['uom-group', groupId], queryFn: () => api<{ data: Row }>(`/api/uom-groups/${groupId}`), enabled: !!groupId, staleTime: 30_000 })
  const ids = allowedUnitIds(product?.unit_id ?? '', groupId ? group.data?.data : null)
  return ids.map((id) => units.find((u) => u.value === id)).filter((o): o is Opt => !!o)
}

/** Cari produk lewat API (bukan memuat semua): ketik minimal 2 huruf SKU/nama/barcode. */
function ProductPicker({ value, onPick, disabled }: { value: Row | null; onPick: (p: Row | null) => void; disabled?: boolean }) {
  const [q, setQ] = useState('')
  const res = useQuery({ queryKey: ['product-search', q], queryFn: () => api<{ data: Row[] }>(`/api/products?q=${encodeURIComponent(q)}&per_page=8&sort=sku&dir=asc`), enabled: q.trim().length >= 2, staleTime: 15_000 })
  if (value) {
    return (
      <div className="flex items-center justify-between gap-2 rounded-xl border border-border bg-surface-2 px-3 py-1.5 text-sm">
        <span className="truncate">{value.sku} · {value.name}</span>
        {!disabled && <button type="button" onClick={() => onPick(null)} className="shrink-0 text-primary-text underline">Ganti</button>}
      </div>
    )
  }
  const rows = res.data?.data ?? []
  return (
    <div>
      <input type="search" aria-label="Cari produk" placeholder="Cari SKU, nama, atau barcode…" disabled={disabled} value={q} onChange={(e) => setQ(e.target.value)} className={cell} />
      {q.trim().length >= 2 && (
        <ul className="mt-1 max-h-40 overflow-auto rounded-xl border border-border bg-surface text-sm">
          {rows.length === 0 && <li className="px-3 py-2 text-muted">{res.isLoading ? 'Mencari…' : 'Produk tidak ditemukan.'}</li>}
          {rows.map((p) => (
            <li key={p.id}><button type="button" onClick={() => { onPick(p); setQ('') }} className="block w-full px-3 py-1.5 text-left hover:bg-surface-2">{p.sku} · {p.name}</button></li>
          ))}
        </ul>
      )}
    </div>
  )
}

function UnitSelect({ product, value, onChange, disabled, invalid }: { product: Row | null; value: string; onChange: (v: string) => void; disabled?: boolean; invalid?: boolean }) {
  const opts = useProductUnits(product)
  const base = product?.unit_id ?? ''
  return (
    <select aria-label="Satuan" aria-invalid={invalid} disabled={disabled || !product} value={value || base} onChange={(e) => onChange(e.target.value === base ? '' : e.target.value)} className={cell}>
      {!product && <option value="">—</option>}
      {opts.map((o) => <option key={o.value} value={o.value}>{o.label.split(' · ')[0]}</option>)}
    </select>
  )
}

function ItemsPanel({ rec, canWrite }: { rec: Row; canWrite: boolean }) {
  const qc = useQueryClient()
  const toast = useToast()
  const [q, setQ] = useState('')
  const [page, setPage] = useState(1)
  const [draft, setDraft] = useState<ItemDraft>(emptyDraft())
  const [product, setProduct] = useState<Row | null>(null)
  const [errors, setErrors] = useState<Record<string, string>>({})
  const [editId, setEditId] = useState<string | null>(null)
  const [edit, setEdit] = useState<ItemDraft>(emptyDraft())
  const [editErrors, setEditErrors] = useState<Record<string, string>>({})
  const [del, setDel] = useState<Row | null>(null)

  const base = `/api/price-lists/${rec.id}/items`
  const key = ['pl-items', rec.id]
  const list = useQuery({ queryKey: [...key, q, page], queryFn: () => api<{ data: Row[]; meta: Row }>(`${base}?per_page=10&page=${page}&q=${encodeURIComponent(q)}`) })
  const rows = list.data?.data ?? []
  const meta = list.data?.meta
  const done = () => qc.invalidateQueries({ queryKey: key })

  const add = useMutation({
    mutationFn: () => api(base, { method: 'POST', body: itemPayload({ ...draft, productId: product?.id ?? '' }, true) }),
    onSuccess: () => { setDraft({ ...emptyDraft(), minQty: draft.minQty }); setProduct(null); setErrors({}); void done(); toast('Item ditambahkan.') },
    onError: (e) => { setErrors(fieldErrors(e)); if (!(e instanceof ApiError && e.status === 422)) toast(e.message, 'error') },
  })
  const save = useMutation({
    mutationFn: () => api(`${base}/${editId}`, { method: 'PUT', body: itemPayload(edit, false) }),
    onSuccess: () => { setEditId(null); setEditErrors({}); void done(); toast('Item disimpan.') },
    onError: (e) => { setEditErrors(fieldErrors(e)); if (!(e instanceof ApiError && e.status === 422)) toast(e.message, 'error') },
  })
  const remove = useMutation({
    mutationFn: (it: Row) => api(`${base}/${it.id}`, { method: 'DELETE' }),
    onSuccess: () => { setDel(null); void done() },
    onError: (e) => { toast(e.message, 'error'); setDel(null) },
  })

  const submitAdd = () => {
    const bad = minQtyError(draft.minQty)
    if (!product) return setErrors({ product_id: 'Produk wajib dipilih.' })
    if (bad) return setErrors({ min_qty: bad })
    setErrors({})
    add.mutate()
  }
  const editProduct = (it: Row): Row => ({ id: it.product_id, unit_id: it.product.unit.id, uom_group_id: it.product.uom_group_id })

  return (
    <div>
      <h3 className="mb-1 font-medium">Item harga</h3>
      <p className="mb-3 text-xs text-muted">Satuan kosong = harga per satuan dasar. Untuk qty tertentu, tier dengan min qty terbesar yang terpenuhi dipakai.</p>

      {canWrite && (
        <div className="mb-4 rounded-xl border border-border p-3">
          <div className="mb-2"><ProductPicker value={product} onPick={(p) => { setProduct(p); setDraft((d) => ({ ...d, unitId: '' })) }} /></div>
          {errors.product_id && <p role="alert" className="mb-2 text-xs text-danger">{errors.product_id}</p>}
          <div className="grid grid-cols-2 gap-2 sm:grid-cols-4">
            <label className="text-xs">Satuan<UnitSelect product={product} value={draft.unitId} onChange={(v) => setDraft({ ...draft, unitId: v })} invalid={!!errors.unit_id} /></label>
            <label className="text-xs">Min qty<input inputMode="numeric" value={draft.minQty} aria-invalid={!!errors.min_qty} onChange={(e) => setDraft({ ...draft, minQty: e.target.value.replace(/\D/g, '') })} className={`${cell} text-right tabular-nums`} /></label>
            <label className="col-span-2 text-xs">Harga<MoneyInput value={draft.price} aria-invalid={!!errors.price} onChange={(v) => setDraft({ ...draft, price: v })} /></label>
          </div>
          {(errors.unit_id || errors.min_qty || errors.price) && <p role="alert" className="mt-2 text-xs text-danger">{errors.unit_id || errors.min_qty || errors.price}</p>}
          <button type="button" onClick={submitAdd} disabled={add.isPending} className="mt-3 inline-flex items-center gap-1.5 rounded-xl bg-primary px-4 py-2 text-sm font-medium text-on-primary hover:bg-primary-hover disabled:opacity-60">
            <Plus size={16} aria-hidden="true" />{add.isPending ? 'Menambah…' : 'Tambah item'}
          </button>
        </div>
      )}

      <input type="search" aria-label="Cari item" placeholder="Cari item (SKU atau nama)…" value={q} onChange={(e) => { setQ(e.target.value); setPage(1) }} className={`${inputCls} mb-2`} />
      <div className="overflow-x-auto rounded-xl border border-border">
        <table className="w-full text-left text-sm">
          <thead className="bg-surface-2 text-xs text-muted">
            <tr><th className="px-2 py-2">Produk</th><th className="px-2 py-2">Satuan</th><th className="px-2 py-2 text-right">Min qty</th><th className="px-2 py-2 text-right">Harga</th><th className="px-2 py-2 text-right">Per satuan dasar</th><th className="px-2 py-2"><span className="sr-only">Aksi</span></th></tr>
          </thead>
          <tbody>
            {list.isLoading && <tr><td colSpan={6} className="px-3 py-4 text-center text-muted">Memuat…</td></tr>}
            {!list.isLoading && rows.length === 0 && <tr><td colSpan={6} className="px-3 py-4 text-center text-muted">..:: Belum ada data ::..</td></tr>}
            {rows.map((it) => editId === it.id ? (
              <tr key={it.id} className="border-t border-border align-top">
                <td className="px-2 py-2">{it.product.sku}<div className="text-xs text-muted">{it.product.name}</div></td>
                <td className="min-w-24 px-2 py-2"><UnitSelect product={editProduct(it)} value={edit.unitId} onChange={(v) => setEdit({ ...edit, unitId: v })} invalid={!!editErrors.unit_id} /></td>
                <td className="px-2 py-2"><input aria-label="Min qty" inputMode="numeric" value={edit.minQty} aria-invalid={!!editErrors.min_qty} onChange={(e) => setEdit({ ...edit, minQty: e.target.value.replace(/\D/g, '') })} className={`${cell} text-right tabular-nums`} /></td>
                <td className="px-2 py-2"><MoneyInput value={edit.price} aria-invalid={!!editErrors.price} onChange={(v) => setEdit({ ...edit, price: v })} /></td>
                <td className="px-2 py-2 text-xs text-danger" colSpan={1}>{editErrors.unit_id || editErrors.min_qty || editErrors.price}</td>
                <td className="whitespace-nowrap px-2 py-2 text-right">
                  <button type="button" aria-label="Simpan item" disabled={save.isPending} onClick={() => minQtyError(edit.minQty) ? setEditErrors({ min_qty: minQtyError(edit.minQty) }) : save.mutate()} className="rounded-lg p-1.5 text-primary-text hover:bg-surface-2"><Check size={16} aria-hidden="true" /></button>
                  <button type="button" aria-label="Batal ubah" onClick={() => { setEditId(null); setEditErrors({}) }} className="rounded-lg p-1.5 text-muted hover:bg-surface-2"><X size={16} aria-hidden="true" /></button>
                </td>
              </tr>
            ) : (
              <tr key={it.id} className="border-t border-border">
                <td className="px-2 py-2">{it.product.sku}<div className="text-xs text-muted">{it.product.name}</div></td>
                <td className="px-2 py-2">{itemUnitCode(it)}</td>
                <td className="px-2 py-2 text-right tabular-nums">{it.min_qty}</td>
                <td className="px-2 py-2 text-right tabular-nums">{formatDecimal(it.price)}</td>
                <td className="px-2 py-2 text-right text-xs tabular-nums text-muted">{perBaseText(it, formatDecimal)}</td>
                <td className="whitespace-nowrap px-2 py-2 text-right">
                  {canWrite && (
                    <>
                      <button type="button" aria-label={`Ubah item ${it.product.sku}`} onClick={() => { setEditId(it.id); setEdit(draftFromItem(it)); setEditErrors({}) }} className="rounded-lg p-1.5 hover:bg-surface-2"><Pencil size={16} aria-hidden="true" /></button>
                      <button type="button" aria-label={`Hapus item ${it.product.sku}`} onClick={() => setDel(it)} className="rounded-lg p-1.5 text-danger hover:bg-surface-2"><Trash2 size={16} aria-hidden="true" /></button>
                    </>
                  )}
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>
      {meta && meta.last_page > 1 && (
        <div className="mt-2 flex items-center justify-between text-sm text-muted">
          <span>{meta.total} item</span>
          <span className="flex items-center gap-2">
            <button type="button" disabled={page <= 1} onClick={() => setPage(page - 1)} className="rounded-lg border border-border px-3 py-1 disabled:opacity-50">Sebelumnya</button>
            {page} / {meta.last_page}
            <button type="button" disabled={page >= meta.last_page} onClick={() => setPage(page + 1)} className="rounded-lg border border-border px-3 py-1 disabled:opacity-50">Berikutnya</button>
          </span>
        </div>
      )}
      <ConfirmDialog open={!!del} title="Hapus item?" message={del ? `Harga ${del.product.sku} (min qty ${del.min_qty}) akan dihapus dari price list ini.` : ''} busy={remove.isPending} onConfirm={() => del && remove.mutate(del)} onCancel={() => setDel(null)} />
    </div>
  )
}

const priceLists: CrudConfig = {
  title: 'Daftar Harga', noun: 'price list', path: '/api/price-lists', permKey: 'admin.price-lists.index',
  columns: [
    { key: 'code', label: 'Kode' }, { key: 'name', label: 'Nama' },
    { key: 'store', label: 'Gerai', render: (r) => r.store?.name ?? 'Semua gerai' },
    { key: 'effective_from', label: 'Masa berlaku', render: (r) => validityText(r.effective_from, r.effective_until) },
    { key: 'is_default', label: 'Default', render: (r) => (r.is_default ? 'Ya' : '—') },
    { key: 'is_active', label: 'Status', render: (r) => <ActiveBadge active={r.is_active} /> },
  ],
  fields: priceListFields,
  wide: true,
  extra: (rec, canWrite) => <ItemsPanel rec={rec} canWrite={canWrite} />,
}
export const PriceListsPage = ({ me }: { me: Me }) => <CrudPage me={me} cfg={priceLists} />
