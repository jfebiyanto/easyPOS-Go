import { Hammer } from 'lucide-react'

export function ComingSoon({ title }: { title: string }) {
  return (
    <div className="mx-auto mt-16 max-w-md rounded-2xl border border-border bg-surface p-8 text-center">
      <Hammer className="mx-auto mb-3 text-muted" size={28} aria-hidden="true" />
      <h1 className="text-lg font-semibold">{title}</h1>
      <p className="mt-1 text-sm text-muted">Segera hadir.</p>
    </div>
  )
}
