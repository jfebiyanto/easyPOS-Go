export const NAV_KEY = 'easypos.nav.collapsed'

/** Daftar label grup yang diciutkan, dari localStorage; nilai rusak = kosong (semua terbuka). */
export function parseCollapsed(raw: string | null): string[] {
  try {
    const v = JSON.parse(raw ?? '[]')
    return Array.isArray(v) ? v.filter((x): x is string => typeof x === 'string') : []
  } catch {
    return []
  }
}

export function toggle(list: string[], label: string): string[] {
  return list.includes(label) ? list.filter((l) => l !== label) : [...list, label]
}
