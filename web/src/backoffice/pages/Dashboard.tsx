import type { Me } from '../../shared/api'

function greeting(h: number): string {
  if (h < 11) return 'Selamat pagi'
  if (h < 15) return 'Selamat siang'
  if (h < 18) return 'Selamat sore'
  return 'Selamat malam'
}

export function Dashboard({ me }: { me: Me }) {
  const now = new Date()
  const date = now.toLocaleDateString('id-ID', { weekday: 'long', day: 'numeric', month: 'long', year: 'numeric' })
  return (
    <div className="rounded-[14px] border border-border bg-surface p-6">
      <h1 className="text-xl font-semibold">{greeting(now.getHours())}, {me.name}</h1>
      <p className="mt-1 text-sm text-muted">{date}</p>
      <p className="mt-4 text-sm text-muted">Ringkasan penjualan dan stok akan tampil di sini pada fase berikutnya.</p>
    </div>
  )
}
