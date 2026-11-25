import { useEffect, useState } from 'react'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { api, type Me } from '../../shared/api'

type Row = {
  transaction_type: string; label: string; prefix: string; padding: number; separator: string
  reset_period: string; last_number: number; example: string
}

const periods: Record<string, string> = { daily: 'Harian', monthly: 'Bulanan', yearly: 'Tahunan', never: 'Tidak pernah' }
const input = 'w-full rounded-lg border border-border bg-surface-2 px-2 py-1.5 text-sm text-text'

export function NumberingsPage({ me }: { me: Me }) {
  const qc = useQueryClient()
  const allowed = me.permissions['admin.numberings.index']?.visible
  const { data, error } = useQuery({ queryKey: ['numberings'], queryFn: () => api<{ numberings: Row[] }>('/api/numberings'), enabled: !!allowed })
  const [rows, setRows] = useState<Row[]>([])
  const [saved, setSaved] = useState(false)
  useEffect(() => { if (data) setRows(data.numberings) }, [data])

  const save = useMutation({
    mutationFn: () => api<{ numberings: Row[] }>('/api/numberings', { method: 'PUT', body: { items: rows.map(({ transaction_type, prefix, padding, separator, reset_period }) => ({ transaction_type, prefix, padding, separator, reset_period })) } }),
    onSuccess: (p) => { qc.setQueryData(['numberings'], p); setSaved(true) },
  })

  if (!allowed) return <p role="alert" className="text-danger">Hanya Owner yang dapat mengatur penomoran.</p>
  if (error) return <p role="alert" className="text-danger">{error.message}</p>
  if (!data) return <p className="text-muted">Memuat…</p>

  const set = (i: number, patch: Partial<Row>) => { setSaved(false); setRows(rows.map((r, j) => (j === i ? { ...r, ...patch } : r))) }

  return (
    <form onSubmit={(e) => { e.preventDefault(); save.mutate() }} className="rounded-[14px] border border-border bg-surface p-4 md:p-6">
      <h1 className="text-xl font-semibold">Penomoran</h1>
      <p className="mt-1 text-sm text-muted">Format nomor: prefix, periode, lalu nomor urut. Contoh menunjukkan nomor berikutnya.</p>
      <div className="mt-4 overflow-x-auto">
        <table className="w-full min-w-[720px] text-left text-sm">
          <thead className="text-muted">
            <tr>{['Tipe', 'Prefix', 'Digit', 'Pemisah', 'Reset', 'Terakhir', 'Contoh'].map((h) => <th key={h} scope="col" className="px-2 py-2 font-medium">{h}</th>)}</tr>
          </thead>
          <tbody>
            {rows.map((r, i) => (
              <tr key={r.transaction_type} className="border-t border-border">
                <th scope="row" className="px-2 py-2 font-medium">{r.label}</th>
                <td className="px-2 py-2"><input aria-label={`Prefix ${r.label}`} className={input} maxLength={20} value={r.prefix} onChange={(e) => set(i, { prefix: e.target.value })} /></td>
                <td className="px-2 py-2"><input aria-label={`Digit ${r.label}`} className={input} type="number" min={1} max={12} value={r.padding} onChange={(e) => set(i, { padding: Number(e.target.value) })} /></td>
                <td className="px-2 py-2"><input aria-label={`Pemisah ${r.label}`} className={input} maxLength={1} value={r.separator} onChange={(e) => set(i, { separator: e.target.value })} /></td>
                <td className="px-2 py-2">
                  <select aria-label={`Reset ${r.label}`} className={input} value={r.reset_period} onChange={(e) => set(i, { reset_period: e.target.value })}>
                    {Object.entries(periods).map(([v, l]) => <option key={v} value={v}>{l}</option>)}
                  </select>
                </td>
                <td className="px-2 py-2 text-muted">{r.last_number}</td>
                <td className="px-2 py-2 font-mono text-xs text-muted">{r.example}</td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>
      {save.error && <p role="alert" className="mt-3 rounded-xl bg-danger-soft px-3 py-2 text-sm text-danger">{save.error.message}</p>}
      {saved && <p role="status" className="mt-3 rounded-xl bg-primary-soft px-3 py-2 text-sm text-primary-text">Penomoran tersimpan.</p>}
      <button type="submit" disabled={save.isPending} className="mt-4 rounded-xl bg-primary px-5 py-2.5 font-medium text-on-primary hover:bg-primary-hover disabled:opacity-60">
        {save.isPending ? 'Menyimpan…' : 'Simpan'}
      </button>
    </form>
  )
}
