import { createContext, useCallback, useContext, useState, type ReactNode } from 'react'
import * as Dialog from '@radix-ui/react-dialog'
import { X } from 'lucide-react'

/* ---------- Toast ---------- */
type Toast = { id: number; kind: 'ok' | 'error'; text: string }
const ToastCtx = createContext<(text: string, kind?: Toast['kind']) => void>(() => {})
export const useToast = () => useContext(ToastCtx)

export function ToastProvider({ children }: { children: ReactNode }) {
  const [list, setList] = useState<Toast[]>([])
  const push = useCallback((text: string, kind: Toast['kind'] = 'ok') => {
    const id = Date.now() + Math.random()
    setList((l) => [...l, { id, kind, text }])
    setTimeout(() => setList((l) => l.filter((t) => t.id !== id)), kind === 'error' ? 7000 : 4000)
  }, [])
  return (
    <ToastCtx.Provider value={push}>
      {children}
      <div className="pointer-events-none fixed bottom-4 right-4 z-[60] flex w-80 max-w-[calc(100vw-2rem)] flex-col gap-2">
        {list.map((t) => (
          <p key={t.id} role={t.kind === 'error' ? 'alert' : 'status'}
            className={`pointer-events-auto rounded-xl border px-4 py-3 text-sm shadow-lg ${t.kind === 'error' ? 'border-danger bg-danger-soft text-danger' : 'border-border bg-primary-soft text-primary-text'}`}>
            {t.text}
          </p>
        ))}
      </div>
    </ToastCtx.Provider>
  )
}

/* ---------- Konfirmasi ---------- */
export function ConfirmDialog({ open, title, message, confirmLabel = 'Hapus', busy, onConfirm, onCancel }: {
  open: boolean; title: string; message: string; confirmLabel?: string; busy?: boolean; onConfirm: () => void; onCancel: () => void
}) {
  return (
    <Dialog.Root open={open} onOpenChange={(o) => !o && onCancel()}>
      <Dialog.Portal>
        <Dialog.Overlay className="fixed inset-0 z-40 bg-black/50" />
        <Dialog.Content className="fixed left-1/2 top-1/2 z-50 w-[calc(100vw-2rem)] max-w-sm -translate-x-1/2 -translate-y-1/2 rounded-2xl border border-border bg-surface p-6 shadow-xl">
          <Dialog.Title className="text-lg font-semibold">{title}</Dialog.Title>
          <Dialog.Description className="mt-2 text-sm text-muted">{message}</Dialog.Description>
          <div className="mt-5 flex justify-end gap-2">
            <button type="button" onClick={onCancel} className="rounded-xl border border-border px-4 py-2 text-sm hover:bg-surface-2">Batal</button>
            <button type="button" onClick={onConfirm} disabled={busy} className="rounded-xl bg-danger px-4 py-2 text-sm font-medium text-bg hover:opacity-90 disabled:opacity-60">
              {busy ? 'Memproses…' : confirmLabel}
            </button>
          </div>
        </Dialog.Content>
      </Dialog.Portal>
    </Dialog.Root>
  )
}

/* ---------- Drawer (panel kanan) ---------- */
export function Drawer({ open, title, description, onClose, children, wide }: {
  open: boolean; title: string; description?: string; onClose: () => void; children: ReactNode; wide?: boolean
}) {
  return (
    <Dialog.Root open={open} onOpenChange={(o) => !o && onClose()}>
      <Dialog.Portal>
        <Dialog.Overlay className="fixed inset-0 z-40 bg-black/50" />
        <Dialog.Content className={`fixed inset-y-0 right-0 z-50 flex w-full ${wide ? 'max-w-2xl' : 'max-w-lg'} flex-col border-l border-border bg-surface shadow-xl`}>
          <div className="flex items-start justify-between gap-3 border-b border-border px-5 py-4">
            <div>
              <Dialog.Title className="text-lg font-semibold">{title}</Dialog.Title>
              <Dialog.Description className={description ? 'mt-0.5 text-sm text-muted' : 'sr-only'}>{description ?? title}</Dialog.Description>
            </div>
            <Dialog.Close className="rounded-lg p-2 hover:bg-surface-2" aria-label="Tutup"><X size={18} aria-hidden="true" /></Dialog.Close>
          </div>
          <div className="flex-1 overflow-y-auto px-5 py-4">{children}</div>
        </Dialog.Content>
      </Dialog.Portal>
    </Dialog.Root>
  )
}
