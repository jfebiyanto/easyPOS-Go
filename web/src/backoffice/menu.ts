import { Boxes, Building2, CalendarCheck, FileText, Hash, LayoutDashboard, Percent, Receipt, RotateCcw, Settings, ShoppingBag, Tags, Truck, Users, type LucideIcon } from 'lucide-react'

export type MenuItem = { key: string; label: string; path: string; icon: LucideIcon }
export type MenuGroup = { label: string; items: MenuItem[] }

// key = route key gaya Laravel; tampil bila me.permissions[key].visible.
export const menu: MenuGroup[] = [
  { label: 'Ringkasan', items: [{ key: 'admin.dashboard.index', label: 'Ringkasan', path: '/', icon: LayoutDashboard }] },
  {
    label: 'Penjualan',
    items: [
      { key: 'admin.sales.index', label: 'Penjualan', path: '/sales', icon: Receipt },
      { key: 'admin.sales-returns.index', label: 'Retur Penjualan', path: '/sales-returns', icon: RotateCcw },
    ],
  },
  {
    label: 'Katalog',
    items: [
      { key: 'admin.products.index', label: 'Produk', path: '/products', icon: ShoppingBag },
      { key: 'admin.price-lists.index', label: 'Daftar Harga', path: '/price-lists', icon: Tags },
      { key: 'admin.customers.index', label: 'Pelanggan', path: '/customers', icon: Users },
      { key: 'admin.promotions.index', label: 'Promosi', path: '/promotions', icon: Percent },
    ],
  },
  {
    label: 'Stok',
    items: [
      { key: 'admin.stock-review.index', label: 'Review Stok', path: '/stock-review', icon: Boxes },
      { key: 'admin.stock-adjustments.index', label: 'Penyesuaian Stok', path: '/stock-adjustments', icon: FileText },
      { key: 'admin.stock-transfer-shipments.index', label: 'Transfer Stok', path: '/stock-transfers', icon: Truck },
    ],
  },
  {
    label: 'Lainnya',
    items: [
      { key: 'admin.eod.index', label: 'End of Day', path: '/eod', icon: CalendarCheck },
      { key: 'admin.numberings.index', label: 'Penomoran', path: '/numberings', icon: Hash },
      { key: 'admin.settings-view.index', label: 'Konfigurasi Sistem', path: '/settings', icon: Settings },
      { key: 'admin.company.edit', label: 'Perusahaan', path: '/company', icon: Building2 },
    ],
  },
]

export const allItems = menu.flatMap((g) => g.items)
