import { StrictMode } from 'react'
import { createRoot } from 'react-dom/client'
import { BrowserRouter } from 'react-router-dom'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import '../shared/theme.css'
import { setUnauthorizedHandler } from '../shared/api'
import { ToastProvider } from '../shared/ui/overlays'
import { App } from './App'

const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false, refetchOnWindowFocus: false } } })
// 401 dari request mana pun → sesi dianggap habis, tampilkan halaman login.
setUnauthorizedHandler(() => queryClient.setQueryData(['me'], null))

createRoot(document.getElementById('root')!).render(
  <StrictMode>
    <QueryClientProvider client={queryClient}>
      <BrowserRouter basename="/backoffice">
        <ToastProvider>
          <App />
        </ToastProvider>
      </BrowserRouter>
    </QueryClientProvider>
  </StrictMode>,
)
