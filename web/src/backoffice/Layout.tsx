import { useState, type ReactNode } from 'react'
import { NavLink, useLocation } from 'react-router-dom'
import * as Dialog from '@radix-ui/react-dialog'
import * as Dropdown from '@radix-ui/react-dropdown-menu'
import { useQuery, useQueryClient } from '@tanstack/react-query'
import { Check, ChevronDown, LogOut, Menu, PanelLeftClose, PanelLeftOpen, Store, X } from 'lucide-react'
import { api, roleLabel, type Me, type StoreRef } from '../shared/api'
import { Logo, LogoMark } from '../shared/Logo'
import { ThemeToggle } from '../shared/ThemeToggle'
import { menu } from './menu'
import { NAV_KEY, RAIL_KEY, parseCollapsed, toggle } from './navState'

function Nav({ me, onNavigate, compact = false }: { me: Me; onNavigate?: () => void; compact?: boolean }) {
  const { pathname } = useLocation()
  const [collapsed, setCollapsed] = useState(() => {
    try { return parseCollapsed(localStorage.getItem(NAV_KEY)) } catch { return [] }
  })
  function flip(label: string) {
    const next = toggle(collapsed, label)
    setCollapsed(next)
    try { localStorage.setItem(NAV_KEY, JSON.stringify(next)) } catch { /* penyimpanan diblokir: state tetap di memori */ }
  }
  return (
    <nav aria-label="Menu utama" className={`flex flex-col gap-3 ${compact ? 'items-center px-2 py-4' : 'p-4'}`}>
      {menu.map((g) => {
        const items = g.items.filter((i) => me.permissions[i.key]?.visible)
        if (!items.length) return null
        // grup yang memuat halaman aktif tidak boleh tersembunyi
        const hasActive = items.some((i) => (i.path === '/' ? pathname === '/' : pathname.startsWith(i.path)))
        const open = compact || hasActive || !collapsed.includes(g.label)
        const single = g.items.length === 1
        return (
          <div key={g.label} className={compact ? 'flex flex-col items-center gap-1 border-b border-border pb-3 last:border-0' : undefined}>
            {!single && !compact && (
              <button type="button" onClick={() => flip(g.label)} aria-expanded={open} aria-controls={`nav-${g.label}`}
                className="mb-1 flex w-full items-center justify-between rounded-lg px-3 py-1 text-xs font-semibold uppercase tracking-wide text-muted hover:bg-surface-2">
                {g.label}
                <ChevronDown size={14} aria-hidden="true" className={`transition-transform ${open ? '' : '-rotate-90'}`} />
              </button>
            )}
            {open && (
              <div id={`nav-${g.label}`} className={compact ? 'flex flex-col items-center gap-1' : undefined}>
                {items.map(({ path, label, icon: Icon }) => (
                  <NavLink key={path} to={path} end={path === '/'} onClick={onNavigate}
                    title={compact ? label : undefined} aria-label={compact ? label : undefined}
                    className={({ isActive }) =>
                      `flex items-center gap-3 rounded-xl py-2 text-sm font-medium ${compact ? 'justify-center px-2.5' : 'px-3'} ${isActive ? 'bg-primary-soft text-primary-text' : 'text-text hover:bg-surface-2'}`}>
                    <Icon size={18} aria-hidden="true" />
                    {!compact && label}
                  </NavLink>
                ))}
              </div>
            )}
          </div>
        )
      })}
    </nav>
  )
}

const menuItem = 'flex cursor-pointer items-center gap-2 rounded-lg px-3 py-2 text-sm text-text outline-none data-[highlighted]:bg-primary-soft'
const menuBox = 'z-50 min-w-56 rounded-xl border border-border bg-surface p-1.5 shadow-lg'

function StoreSwitcher({ me }: { me: Me }) {
  const qc = useQueryClient()
  const { data } = useQuery({ queryKey: ['stores'], queryFn: () => api<{ stores: StoreRef[] }>('/api/stores/accessible'), enabled: me.role === 'owner' })
  const stores = data?.stores ?? []
  if (me.role !== 'owner' || stores.length < 2) {
    return (
      <span className="flex items-center gap-2 rounded-xl border border-border bg-surface px-3 py-1.5 text-sm">
        <Store size={16} aria-hidden="true" /> {me.active_store?.name ?? 'Tanpa gerai'}
      </span>
    )
  }
  async function pick(id: string) {
    await api('/api/session/store', { method: 'POST', body: { store_id: id } })
    await qc.invalidateQueries()
  }
  return (
    <Dropdown.Root>
      <Dropdown.Trigger className="flex items-center gap-2 rounded-xl border border-border bg-surface px-3 py-1.5 text-sm" aria-label="Pilih gerai">
        <Store size={16} aria-hidden="true" /> {me.active_store?.name ?? 'Pilih gerai'} <ChevronDown size={14} aria-hidden="true" />
      </Dropdown.Trigger>
      <Dropdown.Portal>
        <Dropdown.Content align="end" className={menuBox}>
          {stores.map((s) => (
            <Dropdown.Item key={s.id} className={menuItem} onSelect={() => pick(s.id)}>
              <span className="w-4">{s.id === me.active_store?.id && <Check size={14} aria-hidden="true" />}</span>
              {s.code} Â· {s.name}
            </Dropdown.Item>
          ))}
        </Dropdown.Content>
      </Dropdown.Portal>
    </Dropdown.Root>
  )
}

function UserMenu({ me }: { me: Me }) {
  const qc = useQueryClient()
  async function logout() {
    await api('/api/auth/logout', { method: 'POST' }).catch(() => {})
    qc.clear()
    qc.setQueryData(['me'], null)
  }
  return (
    <Dropdown.Root>
      <Dropdown.Trigger className="flex items-center gap-2 rounded-xl border border-border bg-surface px-3 py-1.5 text-sm" aria-label="Menu pengguna">
        {me.name} <ChevronDown size={14} aria-hidden="true" />
      </Dropdown.Trigger>
      <Dropdown.Portal>
        <Dropdown.Content align="end" className={menuBox}>
          <div className="px-3 py-2">
            <p className="text-sm font-medium">{me.name}</p>
            <p className="text-xs text-muted">{roleLabel[me.role]} Â· {me.company.name}</p>
          </div>
          <div className="flex items-center justify-between px-3 py-2 text-sm">
            Tema <ThemeToggle />
          </div>
          <Dropdown.Separator className="my-1 h-px bg-border" />
          <Dropdown.Item className={menuItem} onSelect={logout}>
            <LogOut size={16} aria-hidden="true" /> Keluar
          </Dropdown.Item>
        </Dropdown.Content>
      </Dropdown.Portal>
    </Dropdown.Root>
  )
}

export function Layout({ me, children }: { me: Me; children: ReactNode }) {
  const [open, setOpen] = useState(false)
  const [rail, setRail] = useState(() => {
    try { return localStorage.getItem(RAIL_KEY) === '1' } catch { return false }
  })
  function flipRail() {
    setRail(!rail)
    try { localStorage.setItem(RAIL_KEY, rail ? '0' : '1') } catch { /* diblokir: hanya berlaku di sesi ini */ }
  }
  return (
    <div className="flex h-full bg-bg">
      <aside className={`hidden shrink-0 flex-col overflow-y-auto border-r border-border bg-surface transition-[width] md:flex ${rail ? 'w-16' : 'w-64'}`}>
        <div className={rail ? 'flex justify-center px-2 pt-4' : 'px-4 pt-4'}>
          {rail ? <LogoMark className="h-10 w-10" /> : <Logo className="h-auto w-full" />}
        </div>
        <Nav me={me} compact={rail} />
        <button type="button" onClick={flipRail} aria-pressed={rail} title={rail ? 'Perluas menu' : 'Kecilkan menu'}
          aria-label={rail ? 'Perluas menu samping' : 'Kecilkan menu samping'}
          className={`sticky bottom-0 mt-auto flex items-center gap-3 border-t border-border bg-surface px-5 py-3 text-sm text-muted hover:text-text ${rail ? 'justify-center' : ''}`}>
          {rail ? <PanelLeftOpen size={18} aria-hidden="true" /> : <><PanelLeftClose size={18} aria-hidden="true" /> Kecilkan menu</>}
        </button>
      </aside>
      <div className="flex min-w-0 flex-1 flex-col">
        <header className="flex items-center justify-between gap-3 border-b border-border bg-surface px-4 py-3">
          <Dialog.Root open={open} onOpenChange={setOpen}>
            <Dialog.Trigger className="rounded-xl border border-border p-2 md:hidden" aria-label="Buka menu">
              <Menu size={18} aria-hidden="true" />
            </Dialog.Trigger>
            <Dialog.Portal>
              <Dialog.Overlay className="fixed inset-0 z-40 bg-black/50" />
              <Dialog.Content className="fixed inset-y-0 left-0 z-50 w-72 overflow-y-auto bg-surface">
                <div className="px-4 pt-4"><Logo className="h-auto w-full" /></div>
                <Dialog.Title className="sr-only">easyPOS</Dialog.Title>
                <Dialog.Description className="sr-only">Navigasi menu backoffice</Dialog.Description>
                <Dialog.Close className="absolute right-3 top-3 rounded-lg p-2" aria-label="Tutup menu"><X size={18} aria-hidden="true" /></Dialog.Close>
                <Nav me={me} onNavigate={() => setOpen(false)} />
              </Dialog.Content>
            </Dialog.Portal>
          </Dialog.Root>
          <div className="ml-auto flex items-center gap-2">
            <StoreSwitcher me={me} />
            <UserMenu me={me} />
          </div>
        </header>
        <main className="flex-1 overflow-y-auto p-4 md:p-6">{children}</main>
      </div>
    </div>
  )
}
