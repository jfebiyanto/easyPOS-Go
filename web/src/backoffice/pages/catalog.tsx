import { Plus, Trash2 } from 'lucide-react'
import type { Me } from '../../shared/api'
import { MoneyInput, Select, useOptions } from '../../shared/ui/controls'
import { ActiveBadge, CrudPage, type CrudConfig } from '../../shared/ui/CrudPage'
import { conversionText, type CustomCtx, type FieldDef, type Row } from '../../shared/ui/crudLogic'

const active: FieldDef = { name: 'is_active', label: 'Aktif', type: 'switch', default: true }
const activeCol = { key: 'is_active', label: 'Status', render: (r: Row) => <ActiveBadge active={r.is_active} /> }

/* ---------- Satuan ---------- */
const units: CrudConfig = {
  title: 'Satuan', noun: 'satuan', path: '/api/units', permKey: 'admin.units.index',
  columns: [{ key: 'code', label: 'Kode' }, { key: 'name', label: 'Nama' }, activeCol],
  fields: [
    { name: 'code', label: 'Kode', type: 'text', required: true, maxLength: 20, hint: 'Mis. PCS, DUS.' },
    { name: 'name', label: 'Nama', type: 'text', required: true, maxLength: 100 },
    active,
  ],
}
export const UnitsPage = ({ me }: { me: Me }) => <CrudPage me={me} cfg={units} />

/* ---------- Grup Produk & Merek ---------- */
const productGroups: CrudConfig = {
  title: 'Grup Produk', noun: 'grup produk', path: '/api/product-groups', permKey: 'admin.groups.index',
  columns: [{ key: 'code', label: 'Kode' }, { key: 'name', label: 'Nama' }, { key: 'description', label: 'Deskripsi', render: (r) => r.description ?? '—' }, activeCol],
  fields: [
    { name: 'code', label: 'Kode', type: 'text', required: true, maxLength: 30 },
    { name: 'name', label: 'Nama', type: 'text', required: true, maxLength: 150 },
    { name: 'description', label: 'Deskripsi', type: 'textarea', maxLength: 500 },
    active,
  ],
}
export const ProductGroupsPage = ({ me }: { me: Me }) => <CrudPage me={me} cfg={productGroups} />

const brands: CrudConfig = {
  title: 'Merek', noun: 'merek', path: '/api/brands', permKey: 'admin.brands.index',
  columns: [{ key: 'code', label: 'Kode' }, { key: 'name', label: 'Nama' }, activeCol],
  fields: [
    { name: 'code', label: 'Kode', type: 'text', required: true, maxLength: 30 },
    { name: 'name', label: 'Nama', type: 'text', required: true, maxLength: 150 },
    active,
  ],
}
export const BrandsPage = ({ me }: { me: Me }) => <CrudPage me={me} cfg={brands} />

/* ---------- UoM Group ---------- */
type Line = { unit_id: string; alt_qty: string; base_qty: string }
const unitRef = { path: '/api/units', label: (r: Row) => `${r.code} · ${r.name}` }

function LinesEditor({ values, set, errors, disabled }: CustomCtx) {
  const opts = useOptions(unitRef)
  const code = (id: unknown) => opts.find((o) => o.value === id)?.label.split(' · ')[0] ?? ''
  const baseId = String(values.base_unit_id ?? '')
  const lines = (values.lines as Line[]) ?? []
  const patch = (i: number, p: Partial<Line>) => set(lines.map((l, j) => (j === i ? { ...l, ...p } : l)))
  const cell = 'min-w-0'
  return (
    <fieldset className="mb-4" disabled={disabled}>
      <legend className="mb-1 text-sm font-medium">Satuan alternatif</legend>
      <p className="mb-2 text-xs text-muted">Satuan dasar selalu 1 = 1. Tulis: jumlah satuan alternatif = jumlah satuan dasar.</p>
      {errors.lines && <p role="alert" className="mb-2 text-xs text-danger">{errors.lines}</p>}
      {lines.map((l, i) => {
        const e = (f: string) => errors[`lines.${i}.${f}`]
        return (
          <div key={i} className="mb-3 rounded-xl border border-border bg-surface-2 p-3">
            <div className="grid grid-cols-[1fr_5.5rem_5.5rem_auto] items-start gap-2">
              <div className={cell}>
                <Select aria-label={`Satuan baris ${i + 1}`} aria-invalid={!!e('unit_id')} value={l.unit_id} onChange={(v) => patch(i, { unit_id: v })} options={opts.filter((o) => o.value !== baseId)} />
              </div>
              <div className={cell}><MoneyInput aria-invalid={!!e('alt_qty')} value={l.alt_qty} onChange={(v) => patch(i, { alt_qty: v })} /></div>
              <div className={cell}><MoneyInput aria-invalid={!!e('base_qty')} value={l.base_qty} onChange={(v) => patch(i, { base_qty: v })} /></div>
              <button type="button" onClick={() => set(lines.filter((_, j) => j !== i))} aria-label={`Hapus baris ${i + 1}`} className="rounded-lg p-2 text-danger hover:bg-surface"><Trash2 size={16} aria-hidden="true" /></button>
            </div>
            {(e('unit_id') || e('alt_qty') || e('base_qty')) && <p role="alert" className="mt-1 text-xs text-danger">{e('unit_id') || e('alt_qty') || e('base_qty')}</p>}
            <p className="mt-1 text-sm text-muted">{conversionText(l.alt_qty, code(l.unit_id), l.base_qty, code(baseId))}</p>
          </div>
        )
      })}
      {!disabled && (
        <button type="button" onClick={() => set([...lines, { unit_id: '', alt_qty: '1', base_qty: '' }])} className="flex items-center gap-2 rounded-xl border border-border px-3 py-2 text-sm hover:bg-surface-2">
          <Plus size={16} aria-hidden="true" /> Tambah satuan
        </button>
      )}
    </fieldset>
  )
}

const uomGroups: CrudConfig = {
  title: 'UoM Group', noun: 'UoM group', path: '/api/uom-groups', permKey: 'admin.uom-groups.index',
  columns: [
    { key: 'code', label: 'Kode' }, { key: 'name', label: 'Nama' },
    { key: 'base_unit', label: 'Satuan dasar', render: (r) => r.base_unit?.code ?? '—' },
    {
      key: 'lines', label: 'Konversi',
      render: (r) => {
        const alt = ((r.lines as Row[]) ?? []).filter((l) => !l.is_base)
        if (!alt.length) return '—'
        return alt.map((l) => conversionText(l.alt_qty, l.unit.code, l.base_qty, r.base_unit?.code ?? '')).join('; ')
      },
    },
    activeCol,
  ],
  fields: [
    { name: 'code', label: 'Kode', type: 'text', required: true, maxLength: 20 },
    { name: 'name', label: 'Nama', type: 'text', required: true, maxLength: 100 },
    { name: 'base_unit_id', label: 'Satuan dasar', type: 'select', required: true, ref: unitRef, hint: 'Stok selalu dihitung dalam satuan dasar.' },
    {
      name: 'lines', label: 'Satuan alternatif', type: 'custom', default: [],
      fromRecord: (r) => ((r.lines as Row[]) ?? []).filter((l) => !l.is_base).map((l) => ({ unit_id: l.unit_id, alt_qty: l.alt_qty, base_qty: l.base_qty })),
      render: (c) => <LinesEditor {...c} />,
    },
    active,
  ],
}
export const UomGroupsPage = ({ me }: { me: Me }) => <CrudPage me={me} cfg={uomGroups} />
