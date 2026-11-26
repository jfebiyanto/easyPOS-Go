import { useState, type ReactNode } from 'react'
import { NavLink } from 'react-router-dom'
import * as Dialog from '@radix-ui/react-dialog'
import * as Dropdown from '@radix-ui/react-dropdown-menu'
import { useQuery, useQueryClient } from '@tanstack/react-query'
import { Check, ChevronDown, LogOut, Menu, Store, X } from 'lucide-react'
import { api, roleLabel, type Me, type StoreRef } from '../shared/api'
import { Logo } from '../shared/Logo'
import { ThemeToggle } from '../shared/ThemeToggle'
import { menu } from './menu'

function Nav({ me, onNavigate }: { me: Me; onNavigate?: () => void }) {
  return (
    <nav aria-label="Menu utama" className="flex flex-col gap-5 p-4">
      {menu.map((g) => {
        const items = g.items.filter((i) => me.permissions[i.key]?.visible)
        if (!items.length) return null
        return (
          <div key={g.label}>
            <p className="mb-1 px-3 text-xs font-semibold uppercase tracking-wide text-muted">{g.label}</p>
            {items.map(({ path, label, icon: Icon }) => (
              <NavLink key={path} to={path} end={path === '/'} onClick={onNavigate}
                className={({ isActive }) =>
                  `flex items-center gap-3 rounded-xl px-3 py-2 text-sm font-medium ${isActive ? 'bg-primary-soft text-primary-text' : 'text-text hover:bg-surface-2'}`}>
                <Icon size={18} aria-hidden="true" />
                {label}
              </NavLink>
            ))}
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
  return (
    <div className="flex h-full bg-bg">
      <aside className="hidden w-64 shrink-0 overflow-y-auto border-r border-border bg-surface md:block">
        <div className="px-5 pt-5"><Logo className="h-12" /></div>
        <Nav me={me} />
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
                <div className="px-5 pt-5"><Logo className="h-12" /></div>
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
