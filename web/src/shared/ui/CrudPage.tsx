import { useEffect, useState, type ReactNode } from 'react'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { ArrowDown, ArrowUp, ChevronLeft, ChevronRight, Pencil, Plus, Search, Trash2 } from 'lucide-react'
import { api, ApiError, type Me } from '../api'
import { RecordForm, useOptions } from './controls'
import { buildPayload, fieldErrors, initialValues, nextSort, tabNames, tabsWithErrors, type FieldDef, type Opt, type Row, type Sort, type Values } from './crudLogic'
import { ConfirmDialog, Drawer, useToast } from './overlays'

const cap = (s: string) => s.charAt(0).toUpperCase() + s.slice(1)

export type Column = { key: string; label: string; render?: (r: Row) => ReactNode; className?: string }

export type CrudConfig = {
  title: string
  noun: string
  path: string // mis. /api/tax-groups
  permKey: string // route key halaman (.index), menentukan read-only
  columns: Column[]
  fields: FieldDef[]
  searchHint?: string
  /** Bagian tambahan di drawer saat mengedit (mis. denominasi). */
  extra?: (rec: Row, canWrite: boolean, refresh: () => void) => ReactNode
  canDelete?: (r: Row) => boolean
  /** Filter tambahan di toolbar (dikirim sebagai query ?name=value). */
  filters?: Filter[]
  /** Drawer lebar (formulir bertab). */
  /** Drawer lebar + formulir 2 kolom. Default: otomatis bila field lebih dari 4. */
  wide?: boolean
}

export type Filter = { name: string; all: string; options?: Opt[]; ref?: FieldDef['ref'] }

function FilterSelect({ f, value, onChange }: { f: Filter; value: string; onChange: (v: string) => void }) {
  const remote = useOptions(f.ref ?? { path: '/api/units', label: () => '' }, !!f.ref)
  const opts = f.options ?? remote
  return (
    <label>
      <span className="sr-only">{f.all}</span>
      <select value={value} onChange={(e) => onChange(e.target.value)} className="max-w-44 rounded-xl border border-border bg-surface-2 px-3 py-2 text-sm text-text">
        <option value="">{f.all}</option>
        {opts.map((o) => <option key={o.value} value={o.value}>{o.label}</option>)}
      </select>
    </label>
  )
}

export function ActiveBadge({ active }: { active: boolean }) {
  return (
    <span className={`inline-block rounded-full px-2.5 py-0.5 text-xs font-medium ${active ? 'bg-primary-soft text-primary-text' : 'bg-surface-2 text-muted'}`}>
      {active ? 'Aktif' : 'Nonaktif'}
    </span>
  )
}

type Page = { data: Row[]; meta: { page: number; per_page: number; total: number; last_page: number } }

export function CrudPage({ me, cfg }: { me: Me; cfg: CrudConfig }) {
  const qc = useQueryClient()
  const toast = useToast()
  const canWrite = !(me.permissions[cfg.permKey]?.readOnly ?? true)
  const [q, setQ] = useState('')
  const [search, setSearch] = useState('')
  const [active, setActive] = useState('')
  const [flt, setFlt] = useState<Record<string, string>>({})
  const [page, setPage] = useState(1)
  // default: kolom Kode bila ada, selain itu kolom pertama yang bisa diurutkan
  const sortable = (k: string) => cfg.fields.some((f) => f.name === k && f.type !== 'custom')
  const [sort, setSort] = useState<Sort>({ key: (cfg.columns.find((c) => c.key === 'code') ?? cfg.columns.find((c) => sortable(c.key)))?.key ?? '', dir: 'asc' })
  const [editing, setEditing] = useState<Row | 'new' | null>(null)
  const [toDelete, setToDelete] = useState<Row | null>(null)

  useEffect(() => { const t = setTimeout(() => { setSearch(q); setPage(1) }, 300); return () => clearTimeout(t) }, [q])

  const qs = new URLSearchParams({ page: String(page), per_page: '25' })
  if (search) qs.set('q', search)
  if (active) qs.set('is_active', active)
  for (const [k, v] of Object.entries(flt)) if (v) qs.set(k, v)
  if (sort.key) { qs.set('sort', sort.key); qs.set('dir', sort.dir) }
  const list = useQuery({ queryKey: [cfg.path, 'list', search, active, flt, page, sort], queryFn: () => api<Page>(`${cfg.path}?${qs}`) })
  const refresh = () => qc.invalidateQueries({ queryKey: [cfg.path] })

  const del = useMutation({
    mutationFn: (r: Row) => api(`${cfg.path}/${r.id}`, { method: 'DELETE' }),
    onSuccess: () => { toast(`${cap(cfg.noun)} dihapus.`); setToDelete(null); refresh() },
    onError: (e) => { toast(e.message, 'error'); setToDelete(null) },
  })

  const rows = list.data?.data ?? []
  const meta = list.data?.meta

  return (
    <section className="rounded-[14px] border border-border bg-surface p-4 md:p-6">
      <div className="flex flex-wrap items-center gap-3">
        <h1 className="mr-auto text-xl font-semibold">{cfg.title}</h1>
        <label className="relative">
          <span className="sr-only">Cari</span>
          <Search size={16} aria-hidden="true" className="absolute left-3 top-1/2 -translate-y-1/2 text-muted" />
          <input type="search" value={q} onChange={(e) => setQ(e.target.value)} placeholder={cfg.searchHint ?? 'Cari kode atau nama'}
            className="w-64 rounded-xl border border-border bg-surface-2 py-2 pl-9 pr-3 text-sm text-text" />
        </label>
        <label>
          <span className="sr-only">Status</span>
          <select value={active} onChange={(e) => { setActive(e.target.value); setPage(1) }} className="rounded-xl border border-border bg-surface-2 px-3 py-2 text-sm text-text">
            <option value="">Semua status</option>
            <option value="true">Aktif</option>
            <option value="false">Nonaktif</option>
          </select>
        </label>
        {cfg.filters?.map((f) => <FilterSelect key={f.name} f={f} value={flt[f.name] ?? ''} onChange={(v) => { setFlt({ ...flt, [f.name]: v }); setPage(1) }} />)}
        {canWrite && (
          <button type="button" onClick={() => setEditing('new')} className="flex items-center gap-2 rounded-xl bg-primary px-4 py-2 text-sm font-medium text-on-primary hover:bg-primary-hover">
            <Plus size={16} aria-hidden="true" /> Tambah
          </button>
        )}
      </div>

      {!canWrite && <p className="mt-3 rounded-xl bg-warn-soft px-3 py-2 text-sm text-warn">Anda hanya dapat melihat data ini.</p>}
      {list.error && <p role="alert" className="mt-3 text-danger">{list.error.message}</p>}

      <div className="mt-4 overflow-x-auto">
        <table className="w-full min-w-[560px] text-left text-sm">
          <thead className="border-b border-border text-muted">
            <tr>
              {cfg.columns.map((c) => {
                const on = sort.key === c.key
                return (
                  <th key={c.key} scope="col" aria-sort={on ? (sort.dir === 'asc' ? 'ascending' : 'descending') : undefined} className={`px-3 py-2 font-medium ${c.className ?? ''}`}>
                    {sortable(c.key) ? (
                      <button type="button" onClick={() => { setSort(nextSort(sort, c.key)); setPage(1) }} className={`inline-flex items-center gap-1 font-medium hover:text-text ${on ? 'text-text' : ''}`}>
                        {c.label}
                        {on && (sort.dir === 'asc' ? <ArrowUp size={14} aria-hidden="true" /> : <ArrowDown size={14} aria-hidden="true" />)}
                      </button>
                    ) : c.label}
                  </th>
                )
              })}
              <th scope="col" className="w-24 px-3 py-2"><span className="sr-only">Aksi</span></th>
            </tr>
          </thead>
          <tbody>
            {list.isLoading && <tr><td colSpan={cfg.columns.length + 1} className="px-3 py-6 text-center text-muted">Memuat…</td></tr>}
            {!list.isLoading && rows.length === 0 && <tr><td colSpan={cfg.columns.length + 1} className="px-3 py-6 text-center text-muted">..:: Belum ada data ::..</td></tr>}
            {rows.map((r) => (
              <tr key={r.id} className="border-b border-border last:border-0 hover:bg-surface-2">
                {cfg.columns.map((c) => <td key={c.key} className={`px-3 py-2 ${c.className ?? ''}`}>{c.render ? c.render(r) : String(r[c.key] ?? '—')}</td>)}
                <td className="px-3 py-2">
                  <span className="flex justify-end gap-1">
                    <button type="button" onClick={() => setEditing(r)} aria-label={`${canWrite ? 'Ubah' : 'Lihat'} ${cfg.noun}`} title={canWrite ? 'Ubah' : 'Lihat'} className="rounded-lg p-2 hover:bg-surface"><Pencil size={16} aria-hidden="true" /></button>
                    {canWrite && (cfg.canDelete?.(r) ?? true) && (
                      <button type="button" onClick={() => setToDelete(r)} aria-label={`Hapus ${cfg.noun}`} title="Hapus" className="rounded-lg p-2 text-danger hover:bg-surface"><Trash2 size={16} aria-hidden="true" /></button>
                    )}
                  </span>
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>

      {meta && (
        <div className="mt-3 flex items-center justify-between text-sm text-muted">
          <span>{meta.total} data</span>
          <span className="flex items-center gap-2">
            <button type="button" disabled={page <= 1} onClick={() => setPage(page - 1)} aria-label="Halaman sebelumnya" className="rounded-lg border border-border p-1.5 disabled:opacity-40"><ChevronLeft size={16} aria-hidden="true" /></button>
            Halaman {meta.page} / {meta.last_page}
            <button type="button" disabled={page >= meta.last_page} onClick={() => setPage(page + 1)} aria-label="Halaman berikutnya" className="rounded-lg border border-border p-1.5 disabled:opacity-40"><ChevronRight size={16} aria-hidden="true" /></button>
          </span>
        </div>
      )}

      {editing && (
        <EditDrawer cfg={cfg} target={editing} canWrite={canWrite} onClose={() => setEditing(null)}
          onSaved={() => { setEditing(null); refresh() }} refresh={refresh} />
      )}
      <ConfirmDialog open={!!toDelete} title={`Hapus ${cfg.noun}?`} message="Data yang dihapus tidak bisa dikembalikan." busy={del.isPending}
        onConfirm={() => toDelete && del.mutate(toDelete)} onCancel={() => setToDelete(null)} />
    </section>
  )
}

function EditDrawer({ cfg, target, canWrite, onClose, onSaved, refresh }: {
  cfg: CrudConfig; target: Row | 'new'; canWrite: boolean; onClose: () => void; onSaved: () => void; refresh: () => void
}) {
  const toast = useToast()
  const isCreate = target === 'new'
  const id = isCreate ? null : (target as Row).id
  // Detail dimuat ulang agar field tambahan (mis. denominasi) selalu terbaru.
  const detail = useQuery({ queryKey: [cfg.path, 'detail', id], queryFn: () => api<{ data: Row }>(`${cfg.path}/${id}`), enabled: !!id })
  const rec: Row | null = isCreate ? null : (detail.data?.data ?? null)
  const [values, setValues] = useState<Values>(() => initialValues(cfg.fields, isCreate ? null : (target as Row)))
  const [errors, setErrors] = useState<Record<string, string>>({})
  const [loaded, setLoaded] = useState(isCreate)
  const tabs = tabNames(cfg.fields)
  const [tab, setTab] = useState(tabs[0])
  const errTabs = tabsWithErrors(cfg.fields, errors)
  useEffect(() => {
    if (rec && !loaded) { setValues(initialValues(cfg.fields, rec)); setLoaded(true) }
  }, [rec, loaded, cfg.fields])

  const save = useMutation({
    mutationFn: () => api(isCreate ? cfg.path : `${cfg.path}/${id}`, { method: isCreate ? 'POST' : 'PUT', body: buildPayload(cfg.fields, values, isCreate) }),
    onSuccess: () => { toast(`${cap(cfg.noun)} disimpan.`); onSaved() },
    onError: (e) => {
      const fe = fieldErrors(e)
      setErrors(fe)
      const bad = tabs.find((t) => tabsWithErrors(cfg.fields, fe).has(t))
      if (bad && !tabsWithErrors(cfg.fields, fe).has(tab)) setTab(bad) // lompat ke tab yang bermasalah
      if (!(e instanceof ApiError) || e.status !== 422) toast(e.message, 'error')
    },
  })

  const wide = cfg.wide ?? cfg.fields.filter((f) => f.type !== 'custom' && f.type !== 'textarea').length > 4 // banyak field: 2 kolom
  return (
    <Drawer open wide={wide} title={isCreate ? `Tambah ${cfg.noun}` : `${canWrite ? 'Ubah' : 'Detail'} ${cfg.noun}`} onClose={onClose}>
      {!loaded ? <p className="text-muted">Memuat…</p> : (
        <>
        <form onSubmit={(e) => { e.preventDefault(); setErrors({}); save.mutate() }}>
          {tabs.length > 0 && (
            <div role="tablist" aria-label="Bagian formulir" className="mb-4 flex gap-1 border-b border-border">
              {tabs.map((t) => (
                <button key={t} type="button" role="tab" id={`tab-${t}`} aria-selected={tab === t} onClick={() => setTab(t)}
                  className={`-mb-px border-b-2 px-3 py-2 text-sm font-medium ${tab === t ? 'border-primary text-text' : 'border-transparent text-muted hover:text-text'}`}>
                  {t}{errTabs.has(t) && <span className="ml-1 text-danger" role="img" aria-label="ada kesalahan">•</span>}
                </button>
              ))}
            </div>
          )}
          <RecordForm columns={wide ? 2 : 1} fields={cfg.fields} values={values} errors={errors} isCreate={isCreate} disabled={!canWrite} tab={tab}
            record={rec} refresh={() => { refresh(); detail.refetch() }}
            onChange={(n, v) => { setValues((p) => ({ ...p, [n]: v })); setErrors((p) => (p[n] ? { ...p, [n]: '' } : p)) }} />
          {save.error && !(save.error instanceof ApiError && save.error.status === 422) && <p role="alert" className="mb-3 rounded-xl bg-danger-soft px-3 py-2 text-sm text-danger">{save.error.message}</p>}
          {canWrite && (
            <button type="submit" disabled={save.isPending} className="rounded-xl bg-primary px-5 py-2.5 text-sm font-medium text-on-primary hover:bg-primary-hover disabled:opacity-60">
              {save.isPending ? 'Menyimpan…' : 'Simpan'}
            </button>
          )}
        </form>
        {rec && cfg.extra && <div className="mt-6 border-t border-border pt-4">{cfg.extra(rec, canWrite, () => { refresh(); detail.refetch() })}</div>}
        </>
      )}
    </Drawer>
  )
}
