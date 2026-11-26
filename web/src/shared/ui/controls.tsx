import { useId, useState, type ReactNode } from 'react'
import { useQuery } from '@tanstack/react-query'
import { api } from '../api'
import { cleanMoney, emailError, formatNpwp, npwpError, tabNames, type FieldDef, type Opt, type Row, type Values } from './crudLogic'

export const inputCls =
  'w-full rounded-xl border border-border bg-surface-2 px-3 py-2 text-sm text-text disabled:opacity-60 aria-[invalid=true]:border-danger'

/** Label + kontrol + hint + pesan error, dengan id yang dihubungkan untuk aksesibilitas. */
export function FormField({ label, required, hint, error, full, children }: {
  /** Selebar semua kolom (formulir 2 kolom). */
  full?: boolean
  label: string
  required?: boolean
  hint?: string
  error?: string
  children: (a: { id: string; describedBy?: string; invalid: boolean }) => ReactNode
}) {
  const id = useId()
  const msgId = `${id}-msg`
  const describedBy = error || hint ? msgId : undefined
  return (
    <div className={`mb-4 ${full ? 'sm:col-span-2' : ''}`}>
      <label htmlFor={id} className="mb-1 block text-sm font-medium">
        {label} {required && <span aria-hidden="true" className="text-danger">*</span>}
      </label>
      {children({ id, describedBy, invalid: !!error })}
      {(error || hint) && (
        <p id={msgId} className={`mt-1 text-xs ${error ? 'text-danger' : 'text-muted'}`}>{error || hint}</p>
      )}
    </div>
  )
}

export function Switch({ checked, onChange, id, disabled, label }: {
  checked: boolean; onChange: (v: boolean) => void; id?: string; disabled?: boolean; label: string
}) {
  return (
    <button type="button" role="switch" id={id} aria-checked={checked} aria-label={label} disabled={disabled}
      onClick={() => onChange(!checked)}
      className={`relative h-6 w-11 shrink-0 rounded-full border border-border transition-colors disabled:opacity-60 ${checked ? 'bg-primary' : 'bg-surface-2'}`}>
      <span className={`absolute top-0.5 block size-[18px] rounded-full transition-all ${checked ? 'left-[22px] bg-on-primary' : 'left-0.5 bg-muted'}`} />
    </button>
  )
}

/** Input uang/desimal: nilai selalu string, tidak pernah float. */
export function MoneyInput({ value, onChange, ...rest }: {
  value: string; onChange: (v: string) => void; id?: string; disabled?: boolean; 'aria-invalid'?: boolean; 'aria-describedby'?: string
}) {
  return <input {...rest} inputMode="decimal" autoComplete="off" value={value} onChange={(e) => onChange(cleanMoney(e.target.value))} className={`${inputCls} text-right tabular-nums`} />
}

// shortcut: hanya 100 opsi pertama (batas per_page API); ganti ke pencarian async bila data melebihi itu.
export function useOptions(ref: NonNullable<FieldDef['ref']>, enabled = true): Opt[] {
  const { data } = useQuery({ queryKey: ['options', ref.path], queryFn: () => api<{ data: Row[] }>(`${ref.path}?per_page=100`), staleTime: 30_000, enabled })
  return (data?.data ?? []).map((r) => ({
    value: String(r[ref.value ?? 'id']),
    label: ref.label(r) + (r.is_active === false ? ' (nonaktif)' : ''),
  }))
}

function RefSelect({ f, ...p }: { f: FieldDef; id: string; value: string; onChange: (v: string) => void; disabled?: boolean; 'aria-invalid': boolean; 'aria-describedby'?: string }) {
  const opts = useOptions(f.ref!)
  return <Select {...p} options={opts} />
}

export function Select({ options, value, onChange, ...rest }: {
  options: Opt[]; value: string; onChange: (v: string) => void; id?: string; disabled?: boolean; 'aria-invalid'?: boolean; 'aria-describedby'?: string
}) {
  return (
    <select {...rest} value={value} onChange={(e) => onChange(e.target.value)} className={inputCls}>
      <option value="">— Pilih —</option>
      {options.map((o) => <option key={o.value} value={o.value}>{o.label}</option>)}
    </select>
  )
}

/** Formulir dari definisi field. Field createOnly disembunyikan saat edit; showWhen menyembunyikan secara kondisional. */
export function RecordForm({ fields, values, errors, onChange, isCreate, disabled, tab, record = null, refresh = () => {}, columns = 1 }: {
  columns?: 1 | 2
  fields: FieldDef[]; values: Values; errors: Record<string, string>; onChange: (name: string, v: unknown) => void; isCreate: boolean; disabled?: boolean
  /** Tab aktif; hanya field tab itu yang dirender (field tanpa tab ikut tab pertama). */
  tab?: string
  record?: Row | null
  refresh?: () => void
}) {
  const first = tabNames(fields)[0]
  const [blurErrors, setBlurErrors] = useState<Record<string, string>>({}) // validasi lokal saat lost focus
  return (
    <div className={columns === 2 ? 'grid gap-x-4 sm:grid-cols-2' : ''}>
      {fields.map((f) => {
        if ((f.createOnly && !isCreate) || (f.showWhen && !f.showWhen(values))) return null
        if (tab && (f.tab ?? first) !== tab) return null
        const off = disabled || !!f.disabledWhen?.(values)
        const v = values[f.name]
        const set = (x: unknown) => { if (blurErrors[f.name]) setBlurErrors((p) => ({ ...p, [f.name]: '' })); onChange(f.name, x) }
        const err = errors[f.name] || blurErrors[f.name]
        if (f.type === 'custom') return <div key={f.name} className="sm:col-span-2">{f.render?.({ values, set, errors, disabled: off, setField: onChange, record, refresh })}</div>
        return (
          <FormField key={f.name} full={f.type === 'textarea'} label={f.label} required={f.required} hint={f.hint} error={err}>
            {({ id, describedBy, invalid }) => {
              const a = { id, disabled: off, 'aria-invalid': invalid, 'aria-describedby': describedBy }
              switch (f.type) {
                case 'switch':
                  return <Switch id={id} checked={!!v} onChange={set} disabled={off} label={f.label} />
                case 'money':
                  return <MoneyInput {...a} value={String(v ?? '')} onChange={set} />
                case 'textarea':
                  return <textarea {...a} rows={3} maxLength={f.maxLength} value={String(v ?? '')} onChange={(e) => set(e.target.value)} className={inputCls} />
                case 'select':
                  return f.ref
                    ? <RefSelect f={f} {...a} value={String(v ?? '')} onChange={set} />
                    : <Select {...a} options={f.options ?? []} value={String(v ?? '')} onChange={set} />
                case 'email':
                  return <input {...a} type="email" maxLength={f.maxLength} autoComplete="off" value={String(v ?? '')} onChange={(e) => set(e.target.value)}
                    onBlur={() => setBlurErrors((p) => ({ ...p, [f.name]: emailError(String(v ?? '')) }))} className={inputCls} />
                case 'npwp':
                  return <input {...a} type="text" inputMode="numeric" maxLength={19} placeholder="00.0000.000000.0000" autoComplete="off"
                    value={String(v ?? '')} onChange={(e) => set(e.target.value)} onBlur={() => { const t = String(v ?? ''); onChange(f.name, formatNpwp(t)); setBlurErrors((p) => ({ ...p, [f.name]: npwpError(t) })) }} className={inputCls} />
                default:
                  return (
                    <input {...a} type={f.type === 'number' ? 'number' : f.type} maxLength={f.maxLength}
                      autoComplete={f.type === 'password' ? 'new-password' : 'off'}
                      value={String(v ?? '')} onChange={(e) => set(e.target.value)} className={inputCls} />
                  )
              }
            }}
          </FormField>
        )
      })}
    </div>
  )
}
