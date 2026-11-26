import { useQuery, useQueryClient } from '@tanstack/react-query'
import { Route, Routes } from 'react-router-dom'
import { api, type Me } from '../shared/api'
import { IdleLock } from '../shared/IdleLock'
import { Login } from '../shared/Login'
import { Layout } from './Layout'
import { allItems } from './menu'
import { ComingSoon } from './pages/ComingSoon'
import { Dashboard } from './pages/Dashboard'
import { NumberingsPage } from './pages/Numberings'
import { BrandsPage, ProductGroupsPage, UnitsPage, UomGroupsPage } from './pages/catalog'
import { CompanyPage, CurrenciesPage, StoresPage, TaxGroupsPage, UsersPage, WarehousesPage } from './pages/org'
import { SettingsPage } from './pages/Settings'

export function App() {
  const qc = useQueryClient()
  const { data: me, isLoading } = useQuery({ queryKey: ['me'], queryFn: () => api<Me>('/api/auth/me') })

  if (isLoading) return <p className="p-6 text-muted">Memuat…</p>
  if (!me) return <Login client="backoffice" title="Masuk ke Backoffice" onDone={(u) => qc.setQueryData(['me'], u)} />

  async function logout() {
    await api('/api/auth/logout', { method: 'POST' }).catch(() => {})
    qc.clear()
    qc.setQueryData(['me'], null)
  }

  return (
    <IdleLock me={me} onLogout={logout}>
    <Layout me={me}>
      <Routes>
        <Route path="/" element={<Dashboard me={me} />} />
        <Route path="/settings" element={<SettingsPage me={me} />} />
        <Route path="/numberings" element={<NumberingsPage me={me} />} />
        <Route path="/company" element={<CompanyPage me={me} />} />
        <Route path="/stores" element={<StoresPage me={me} />} />
        <Route path="/warehouses" element={<WarehousesPage me={me} />} />
        <Route path="/users" element={<UsersPage me={me} />} />
        <Route path="/tax-groups" element={<TaxGroupsPage me={me} />} />
        <Route path="/currencies" element={<CurrenciesPage me={me} />} />
        <Route path="/units" element={<UnitsPage me={me} />} />
        <Route path="/uom-groups" element={<UomGroupsPage me={me} />} />
        <Route path="/product-groups" element={<ProductGroupsPage me={me} />} />
        <Route path="/brands" element={<BrandsPage me={me} />} />
        {allItems.map((i) => (
          <Route key={i.path} path={i.path} element={<ComingSoon title={i.label} />} />
        ))}
        <Route path="*" element={<ComingSoon title="Halaman tidak ditemukan" />} />
      </Routes>
    </Layout>
    </IdleLock>
  )
}
