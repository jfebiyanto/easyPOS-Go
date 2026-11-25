export type ThemePref = 'system' | 'light' | 'dark'
export type Theme = 'light' | 'dark'

export const STORAGE_KEY = 'easypos.theme'

/** Pilihan pengguna + preferensi sistem → tema yang dipakai. */
export function resolveTheme(pref: ThemePref, systemDark: boolean): Theme {
  if (pref === 'light' || pref === 'dark') return pref
  return systemDark ? 'dark' : 'light'
}

export function parsePref(raw: string | null): ThemePref {
  return raw === 'light' || raw === 'dark' ? raw : 'system'
}

export function readPref(): ThemePref {
  try {
    return parsePref(localStorage.getItem(STORAGE_KEY))
  } catch {
    return 'system'
  }
}

export function savePref(pref: ThemePref): void {
  try {
    if (pref === 'system') localStorage.removeItem(STORAGE_KEY)
    else localStorage.setItem(STORAGE_KEY, pref)
  } catch {
    /* penyimpanan diblokir: tema tetap berlaku untuk sesi ini */
  }
}

export function systemPrefersDark(): boolean {
  return typeof matchMedia === 'function' && matchMedia('(prefers-color-scheme: dark)').matches
}

export function applyTheme(pref: ThemePref): Theme {
  const t = resolveTheme(pref, systemPrefersDark())
  document.documentElement.setAttribute('data-theme', t)
  return t
}
