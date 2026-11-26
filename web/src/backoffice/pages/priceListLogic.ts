import type { Row } from '../../shared/ui/crudLogic'

export type ItemDraft = { productId: string; unitId: string; minQty: string; price: string }

export const emptyDraft = (): ItemDraft => ({ productId: '', unitId: '', minQty: '1', price: '' })

/** Masa berlaku price list untuk tabel: "Selalu", "Mulai …", "s/d …", atau "… s/d …". */
export function validityText(from?: string | null, until?: string | null): string {
  if (from && until) return `${from} s/d ${until}`
  if (from) return `Mulai ${from}`
  if (until) return `s/d ${until}`
  return 'Selalu'
}

/** Pesan salah untuk min qty (bilangan bulat >= 1); '' bila valid. */
export const minQtyError = (s: string): string => (/^\d+$/.test(s.trim()) && Number(s) >= 1 ? '' : 'Harus bilangan bulat 1 atau lebih.')

/** Satuan item untuk tampilan: satuan khusus bila ada, selain itu satuan dasar produk. */
export const itemUnitCode = (it: Row): string => it.unit?.code ?? it.product?.unit?.code ?? ''

/** Body POST/PUT item. unit kosong = satuan dasar (null). Produk hanya dikirim saat tambah. */
export function itemPayload(d: ItemDraft, isCreate: boolean): Row {
  const body: Row = { unit_id: d.unitId || null, min_qty: Number(d.minQty), price: d.price }
  if (isCreate) body.product_id = d.productId
  return body
}

/** Draft untuk mengedit baris yang ada. */
export const draftFromItem = (it: Row): ItemDraft => ({ productId: it.product_id, unitId: it.unit_id ?? '', minQty: String(it.min_qty), price: String(it.price) })

/** Teks bantu harga per satuan dasar; kosong bila sama dengan harga (satuan dasar). */
export const perBaseText = (it: Row, fmt: (s: string) => string): string =>
  it.unit_id ? `${fmt(it.price_per_base)} / ${it.product.unit.code}` : '—'
