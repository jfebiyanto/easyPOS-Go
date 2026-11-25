export class ApiError extends Error {
  constructor(public status: number, message: string) {
    super(message)
  }
}

let onUnauthorized: () => void = () => {}
export function setUnauthorizedHandler(fn: () => void) {
  onUnauthorized = fn
}

/** Satu-satunya pintu request ke backend: cookie sesi + header CSRF. */
export async function api<T = unknown>(path: string, opts: { method?: string; body?: unknown } = {}): Promise<T> {
  const res = await fetch(path, {
    method: opts.method ?? 'GET',
    credentials: 'include',
    headers: { 'X-EasyPOS-Client': '1', ...(opts.body !== undefined ? { 'Content-Type': 'application/json' } : {}) },
    body: opts.body !== undefined ? JSON.stringify(opts.body) : undefined,
  })
  if (res.status === 204) return undefined as T
  const data = await res.json().catch(() => ({}))
  if (res.status === 401 && path !== '/api/auth/login') onUnauthorized()
  if (!res.ok) throw new ApiError(res.status, (data as { message?: string }).message ?? 'Terjadi kesalahan.')
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
}

export const roleLabel: Record<Me['role'], string> = { owner: 'Owner', store_manager: 'Store Manager', cashier: 'Kasir' }
