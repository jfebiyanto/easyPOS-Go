import { useState, type FormEvent } from 'react'
import { api, type Me } from './api'
import { Logo } from './Logo'
import { ThemeToggle } from './ThemeToggle'

export function Login({ client, title, onDone }: { client: 'backoffice' | 'pos'; title: string; onDone: (me: Me) => void }) {
  const [username, setUsername] = useState('')
  const [password, setPassword] = useState('')
  const [error, setError] = useState('')
  const [busy, setBusy] = useState(false)

  async function submit(e: FormEvent) {
    e.preventDefault()
    setBusy(true)
    setError('')
    try {
      const r = await api<{ user: Me }>('/api/auth/login', { method: 'POST', body: { username, password, client } })
      onDone(r.user)
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Gagal masuk.')
    } finally {
      setBusy(false)
    }
  }

  return (
    <main className="flex min-h-full items-center justify-center bg-bg p-4">
      <div className="absolute right-4 top-4">
        <ThemeToggle />
      </div>
      <form onSubmit={submit} className="w-full max-w-sm rounded-2xl border border-border bg-surface p-6 shadow-sm">
        <Logo className="mb-3 h-14" />
        <h1 className="mb-5 text-sm text-muted">{title}</h1>
        <label htmlFor="username" className="mb-1 block text-sm font-medium">Username</label>
        <input id="username" autoComplete="username" required value={username} onChange={(e) => setUsername(e.target.value)}
          className="mb-3 w-full rounded-xl border border-border bg-surface-2 px-3 py-2 text-text" />
        <label htmlFor="password" className="mb-1 block text-sm font-medium">Password</label>
        <input id="password" type="password" autoComplete="current-password" required value={password} onChange={(e) => setPassword(e.target.value)}
          className="mb-4 w-full rounded-xl border border-border bg-surface-2 px-3 py-2 text-text" />
        {error && <p role="alert" className="mb-3 rounded-xl bg-danger-soft px-3 py-2 text-sm text-danger">{error}</p>}
        <button type="submit" disabled={busy}
          className="w-full rounded-xl bg-primary px-4 py-2.5 font-medium text-on-primary hover:bg-primary-hover disabled:opacity-60">
          {busy ? 'Memproses…' : 'Masuk'}
        </button>
      </form>
    </main>
  )
}
