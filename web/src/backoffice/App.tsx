import { useQuery, useQueryClient } from '@tanstack/react-query'
import { Route, Routes } from 'react-router-dom'
import { api, type Me } from '../shared/api'
import { Login } from '../shared/Login'
import { Layout } from './Layout'
import { allItems } from './menu'
import { ComingSoon } from './pages/ComingSoon'
import { Dashboard } from './pages/Dashboard'
import { NumberingsPage } from './pages/Numberings'
import { SettingsPage } from './pages/Settings'

export function App() {
  const qc = useQueryClient()
  const { data: me, isLoading } = useQuery({ queryKey: ['me'], queryFn: () => api<Me>('/api/auth/me') })

  if (isLoading) return <p className="p-6 text-muted">Memuat…</p>
  if (!me) return <Login client="backoffice" title="Masuk ke Backoffice" onDone={(u) => qc.setQueryData(['me'], u)} />

  return (
    <Layout me={me}>
      <Routes>
        <Route path="/" element={<Dashboard me={me} />} />
        <Route path="/settings" element={<SettingsPage me={me} />} />
        <Route path="/numberings" element={<NumberingsPage me={me} />} />
        {allItems.map((i) => (
          <Route key={i.path} path={i.path} element={<ComingSoon title={i.label} />} />
        ))}
        <Route path="*" element={<ComingSoon title="Halaman tidak ditemukan" />} />
      </Routes>
    </Layout>
  )
}
