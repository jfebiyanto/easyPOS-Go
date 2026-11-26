import { useCallback, useEffect, useRef, useState, type FormEvent, type ReactNode } from 'react'
import { LogoMark } from './Logo'
import { api, setLockedHandler, type Me } from './api'
import { isIdle, needsHeartbeat } from './idle'

const EVENTS = ['pointerdown', 'keydown', 'wheel', 'touchstart'] as const

/** Mengunci layar saat idle (batas dari server, sama untuk semua gerai). Isi halaman tetap ter-mount agar keranjang POS tidak hilang. */
export function IdleLock({ me, onLogout, children }: { me: Me; onLogout: () => void; children: ReactNode }) {
  const [locked, setLocked] = useState(me.locked)
  const last = useRef(Date.now())
  const beat = useRef(Date.now())
  const body = useRef<HTMLDivElement>(null)

  useEffect(() => { setLockedHandler(() => setLocked(true)); return () => setLockedHandler(() => {}) }, [])
  useEffect(() => { body.current?.toggleAttribute('inert', locked) }, [locked])
  useEffect(() => {
    const mark = () => { last.current = Date.now() }
    EVENTS.forEach((e) => addEventListener(e, mark, { passive: true }))
    const t = setInterval(() => {
      const now = Date.now()
      if (isIdle(now, last.current, me.idle_lock_minutes)) { setLocked(true); return }
      if (needsHeartbeat(now, last.current, beat.current)) {
        beat.current = now
        api('/api/auth/activity', { method: 'POST' }).catch(() => {})
      }
    }, 5_000)
    return () => { EVENTS.forEach((e) => removeEventListener(e, mark)); clearInterval(t) }
  }, [me.idle_lock_minutes])

  const unlocked = useCallback(() => { last.current = Date.now(); beat.current = Date.now(); setLocked(false) }, [])

  return (
    <>
      <div ref={body} className="h-full" aria-hidden={locked}>{children}</div>
      {locked && <LockScreen me={me} onUnlocked={unlocked} onLogout={onLogout} />}
    </>
  )
}

function LockScreen({ me, onUnlocked, onLogout }: { me: Me; onUnlocked: () => void; onLogout: () => void }) {
  const [password, setPassword] = useState('')
  const [error, setError] = useState('')
  const [busy, setBusy] = useState(false)

  async function submit(e: FormEvent) {
    e.preventDefault()
    setBusy(true)
    setError('')
    try {
      await api('/api/auth/unlock', { method: 'POST', body: { password } })
      setPassword('')
      onUnlocked()
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Gagal membuka kunci.')
    } finally {
      setBusy(false)
    }
  }

  return (
    <div role="dialog" aria-modal="true" aria-labelledby="lock-title" className="fixed inset-0 z-50 flex items-center justify-center bg-bg p-4">
      <form onSubmit={submit} className="w-full max-w-sm rounded-2xl border border-border bg-surface p-6 shadow-sm">
        <LogoMark className="mb-3 h-14" />
        <h1 id="lock-title" className="text-xl font-semibold text-text">Sesi terkunci</h1>
        <p className="mb-5 mt-1 text-sm text-muted">Terkunci karena tidak ada aktivitas. Masukkan password {me.name} untuk melanjutkan.</p>
        <label htmlFor="unlock-password" className="mb-1 block text-sm font-medium">Password</label>
        <input id="unlock-password" type="password" autoComplete="current-password" autoFocus required value={password}
          onChange={(e) => setPassword(e.target.value)} className="mb-4 w-full rounded-xl border border-border bg-surface-2 px-3 py-2 text-text" />
        {error && <p role="alert" className="mb-3 rounded-xl bg-danger-soft px-3 py-2 text-sm text-danger">{error}</p>}
        <button type="submit" disabled={busy}
          className="w-full rounded-xl bg-primary px-4 py-2.5 font-medium text-on-primary hover:bg-primary-hover disabled:opacity-60">
          {busy ? 'Memproses…' : 'Buka kunci'}
        </button>
        <button type="button" onClick={onLogout} className="mt-2 w-full rounded-xl border border-border px-4 py-2.5 text-text hover:bg-surface-2">
          Keluar
        </button>
      </form>
    </div>
  )
}
