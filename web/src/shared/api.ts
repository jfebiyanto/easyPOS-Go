export class ApiError extends Error {
  constructor(public status: number, message: string, public errors: Record<string, string[]> = {}) {
    super(message)
  }
}

let onUnauthorized: () => void = () => {}
export function setUnauthorizedHandler(fn: () => void) {
  onUnauthorized = fn
}

let onLocked: () => void = () => {}
export function setLockedHandler(fn: () => void) {
  onLocked = fn
}

/** Satu-satunya pintu request ke backend: cookie sesi + header CSRF. */
export async function api<T = unknown>(path: string, opts: { method?: string; body?: unknown } = {}): Promise<T> {
  const isForm = typeof FormData !== 'undefined' && opts.body instanceof FormData
  const res = await fetch(path, {
    method: opts.method ?? 'GET',
    credentials: 'include',
    // FormData: biarkan browser mengisi Content-Type (boundary multipart).
    headers: { 'X-EasyPOS-Client': '1', ...(opts.body !== undefined && !isForm ? { 'Content-Type': 'application/json' } : {}) },
    body: opts.body === undefined ? undefined : isForm ? (opts.body as FormData) : JSON.stringify(opts.body),
  })
  if (res.status === 204) return undefined as T
  const data = await res.json().catch(() => ({}))
  if (res.status === 401 && path !== '/api/auth/login') onUnauthorized()
  if (res.status === 423) onLocked()
  if (!res.ok) {
    const d = data as { message?: string; errors?: Record<string, string[]> }
    throw new ApiError(res.status, d.message ?? 'Terjadi kesalahan.', d.errors)
  }
  return data as T
}

export type StoreRef = { id: string; code: string; name: string }
export type Permission = { visible: boolean; readOnly: boolean }
export type Me = {
  id: string
  name: string
  username: string
  email: string | null
  role: 'owner' | 'store_manager' | 'cashier'
  store_id: string | null
  active_store: StoreRef | null
  company: { id: string; name: string }
  permissions: Record<string, Permission>
  locked: boolean
  idle_lock_minutes: number
}

export const roleLabel: Record<Me['role'], string> = { owner: 'Owner', store_manager: 'Store Manager', cashier: 'Kasir' }
