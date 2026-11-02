import { StrictMode } from 'react'
import { createRoot } from 'react-dom/client'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { BrowserRouter } from 'react-router-dom'
import App from './App'
import './index.css'

const queryClient = new QueryClient({
  // 不再设置全局 refetchInterval：WS 已实时推送 metrics / probe / alert，
  // 各页面按自己的数据新鲜度需求显式声明 refetchInterval，
  // 避免「每个页面每 30s 无条件打一遍 REST」。
  defaultOptions: { queries: { staleTime: 15000 } },
})

createRoot(document.getElementById('root')!).render(
  <StrictMode>
    <QueryClientProvider client={queryClient}>
      <BrowserRouter>
        <App />
      </BrowserRouter>
    </QueryClientProvider>
  </StrictMode>,
)
