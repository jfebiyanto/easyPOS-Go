import type { Row } from '../../shared/ui/crudLogic'

export type BarcodeRow = { id?: string; code: string; unit_id: string; note: string; source: string }

/** Baris barcode satuan dari detail produk, siap diedit. */
export const barcodeRows = (rec: Row): BarcodeRow[] =>
  ((rec.barcodes as Row[]) ?? []).map((b) => ({ id: b.id, code: b.code, unit_id: b.unit_id, note: b.note ?? '', source: b.source ?? 'local' }))

/** Satuan yang boleh dipakai produk: satuan dasar + anggota UoM group (urutan: dasar dulu). */
export function allowedUnitIds(baseUnitId: string, group?: Row | null): string[] {
  if (!baseUnitId) return []
  const ids = [baseUnitId]
  for (const l of (group?.lines as Row[]) ?? []) if (!ids.includes(l.unit_id)) ids.push(l.unit_id)
  return ids
}

export const newBarcodeRow = (unitId: string): BarcodeRow => ({ code: '', unit_id: unitId, note: '', source: 'local' })
