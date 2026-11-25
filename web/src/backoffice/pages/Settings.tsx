import { useEffect, useState } from 'react'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { api, type Me } from '../../shared/api'

type Definition = { key: string; label: string; type: string; options: string[]; default: string }
type Payload = { settings: Record<string, string>; definitions: Definition[] }

const optionLabel: Record<string, string> = {
  reject: 'Tolak — penjualan ditolak bila serial tidak ada di stok',
  manager_approval: 'Perlu persetujuan manajer',
  allow: 'Izinkan — serial boleh dijual walau tidak tercatat di stok',
}

export function SettingsPage({ me }: { me: Me }) {
  const qc = useQueryClient()
  const readOnly = me.permissions['admin.settings-view.index']?.readOnly ?? true
  const { data, error } = useQuery({ queryKey: ['settings'], queryFn: () => api<Payload>('/api/settings') })
  const [draft, setDraft] = useState<Record<string, string>>({})
  const [saved, setSaved] = useState(false)
  useEffect(() => { if (data) setDraft(data.settings) }, [data])

  const save = useMutation({
    mutationFn: () => api<Payload>('/api/settings', { method: 'PUT', body: { settings: draft } }),
    onSuccess: (p) => { qc.setQueryData(['settings'], p); setSaved(true) },
  })

  if (error) return <p role="alert" className="text-danger">{error.message}</p>
  if (!data) return <p className="text-muted">Memuat…</p>

  return (
    <form onSubmit={(e) => { e.preventDefault(); setSaved(false); save.mutate() }} className="max-w-2xl rounded-[14px] border border-border bg-surface p-6">
      <h1 className="text-xl font-semibold">Konfigurasi Sistem</h1>
      {readOnly && <p className="mt-2 rounded-xl bg-warn-soft px-3 py-2 text-sm text-warn">Hanya Owner yang dapat mengubah konfigurasi.</p>}
      {data.definitions.map((d) => (
        <fieldset key={d.key} className="mt-5" disabled={readOnly}>
          <legend className="mb-2 text-sm font-medium">{d.label} <code className="text-xs text-muted">{d.key}</code></legend>
          {d.options.map((o) => (
            <label key={o} className="mb-2 flex cursor-pointer items-start gap-3 rounded-xl border border-border bg-surface-2 p-3 text-sm">
              <input type="radio" name={d.key} value={o} checked={draft[d.key] === o} onChange={() => setDraft({ ...draft, [d.key]: o })} className="mt-0.5" />
              {optionLabel[o] ?? o}
            </label>
          ))}
        </fieldset>
      ))}
      {save.error && <p role="alert" className="mt-3 rounded-xl bg-danger-soft px-3 py-2 text-sm text-danger">{save.error.message}</p>}
      {saved && <p role="status" className="mt-3 rounded-xl bg-primary-soft px-3 py-2 text-sm text-primary-text">Konfigurasi tersimpan.</p>}
      {!readOnly && (
        <button type="submit" disabled={save.isPending} className="mt-4 rounded-xl bg-primary px-5 py-2.5 font-medium text-on-primary hover:bg-primary-hover disabled:opacity-60">
          {save.isPending ? 'Menyimpan…' : 'Simpan'}
        </button>
      )}
    </form>
  )
}
