import { useEffect, useState } from 'react'
import { Monitor, Moon, Sun } from 'lucide-react'
import { applyTheme, readPref, savePref, type ThemePref } from './theme'

const options: { value: ThemePref; label: string; icon: typeof Sun }[] = [
  { value: 'system', label: 'Ikuti sistem', icon: Monitor },
  { value: 'light', label: 'Terang', icon: Sun },
  { value: 'dark', label: 'Gelap', icon: Moon },
]

export function ThemeToggle() {
  const [pref, setPref] = useState<ThemePref>(readPref)

  useEffect(() => {
    applyTheme(pref)
    if (pref !== 'system') return
    const mq = matchMedia('(prefers-color-scheme: dark)')
    const on = () => applyTheme('system')
    mq.addEventListener('change', on)
    return () => mq.removeEventListener('change', on)
  }, [pref])

  return (
    <div role="radiogroup" aria-label="Tema tampilan" className="inline-flex rounded-xl border border-border bg-surface-2 p-0.5">
      {options.map(({ value, label, icon: Icon }) => (
        <button
          key={value}
          type="button"
          role="radio"
          aria-checked={pref === value}
          aria-label={label}
          title={label}
          onClick={() => {
            savePref(value)
            setPref(value)
          }}
          className={`flex h-8 w-8 items-center justify-center rounded-[10px] ${pref === value ? 'bg-primary text-on-primary' : 'text-muted hover:bg-primary-soft'}`}
        >
          <Icon size={16} aria-hidden="true" />
        </button>
      ))}
    </div>
  )
}
