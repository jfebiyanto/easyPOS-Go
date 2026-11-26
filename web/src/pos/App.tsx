import { useEffect, useState } from 'react'
import { useQuery, useQueryClient } from '@tanstack/react-query'
import { LogOut, ShoppingCart, Wifi, WifiOff } from 'lucide-react'
import { api, type Me } from '../shared/api'
import { IdleLock } from '../shared/IdleLock'
import { Login } from '../shared/Login'
import { ThemeToggle } from '../shared/ThemeToggle'

function useOnline() {
  const [online, setOnline] = useState(navigator.onLine)
  useEffect(() => {
    const on = () => setOnline(true)
    const off = () => setOnline(false)
    addEventListener('online', on)
    addEventListener('offline', off)
    return () => { removeEventListener('online', on); removeEventListener('offline', off) }
  }, [])
  return online
}

function Clock() {
  const [now, setNow] = useState(new Date())
  useEffect(() => { const t = setInterval(() => setNow(new Date()), 10_000); return () => clearInterval(t) }, [])
  return <time dateTime={now.toISOString()} className="tabular-nums">{now.toLocaleTimeString('id-ID', { hour: '2-digit', minute: '2-digit' })}</time>
}

export function App() {
  const qc = useQueryClient()
  const online = useOnline()
  const { data: me, isLoading } = useQuery({ queryKey: ['me'], queryFn: () => api<Me>('/api/auth/me') })

  if (isLoading) return <p className="p-6 text-muted">Memuat…</p>
  if (!me) return <Login client="pos" title="Masuk ke Kasir" onDone={(u) => qc.setQueryData(['me'], u)} />

  async function logout() {
    await api('/api/auth/logout', { method: 'POST' }).catch(() => {})
    qc.clear()
    qc.setQueryData(['me'], null)
  }

  return (
    <IdleLock me={me} onLogout={logout}>
    <div className="flex h-full flex-col bg-bg">
      <header className="flex flex-wrap items-center gap-x-4 gap-y-2 border-b border-border bg-surface px-4 py-3">
        <span className="flex items-center gap-2 font-bold"><ShoppingCart size={18} aria-hidden="true" /> easyPOS</span>
        <span className="text-sm text-muted">{me.active_store?.name ?? 'Tanpa gerai'} · {me.name}</span>
        <span className="ml-auto flex items-center gap-4 text-sm">
          <span className={`flex items-center gap-1.5 ${online ? 'text-text' : 'text-warn'}`}>
            {online ? <Wifi size={16} aria-hidden="true" /> : <WifiOff size={16} aria-hidden="true" />}
            {online ? 'Online' : 'Offline'}
          </span>
          <Clock />
          <ThemeToggle />
          <button type="button" onClick={logout} className="flex items-center gap-2 rounded-xl border border-border px-3 py-1.5 hover:bg-surface-2">
            <LogOut size={16} aria-hidden="true" /> Keluar
          </button>
        </span>
      </header>
      <main className="flex flex-1 items-center justify-center p-6">
        <p className="rounded-2xl border border-dashed border-border bg-surface px-8 py-10 text-muted">POS akan dibangun di fase 3.</p>
      </main>
    </div>
    </IdleLock>
  )
}
