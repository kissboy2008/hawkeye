import { useState, useCallback, lazy, Suspense } from 'react'
import { Routes, Route } from 'react-router-dom'
import Layout from './components/Layout'

const Dashboard = lazy(() => import('./pages/Dashboard'))
const MachineDetail = lazy(() => import('./pages/MachineDetail'))
const MachineManage = lazy(() => import('./pages/MachineManage'))
const HostStatus = lazy(() => import('./pages/HostStatus'))
const ServiceStatus = lazy(() => import('./pages/ServiceStatus'))
const ProbeDetail = lazy(() => import('./pages/ProbeDetail'))
const AlertConfig = lazy(() => import('./pages/AlertConfig'))
const HawkeyeWidgets = lazy(() => import('./pages/HawkeyeWidgets'))
const LoginPage = lazy(() => import('./pages/LoginPage'))

function PageFallback() {
  return (
    <div className="flex items-center justify-center h-64">
      <div className="w-8 h-8 border-2 border-accent border-t-transparent rounded-full animate-spin" />
    </div>
  )
}

export default function App() {
  const [loggedIn, setLoggedIn] = useState(() => !!localStorage.getItem('auth_token'))

  const handleLogin = useCallback(() => setLoggedIn(true), [])

  if (!loggedIn) {
    return <LoginPage onLogin={handleLogin} />
  }

  return (
    <Suspense fallback={<PageFallback />}>
      <Routes>
        <Route element={<Layout />}>
          <Route path="/" element={<Dashboard />} />
          <Route path="/machine/:name" element={<MachineDetail />} />
          <Route path="/hosts" element={<HostStatus />} />
          <Route path="/services" element={<ServiceStatus />} />
          <Route path="/probe/:id" element={<ProbeDetail />} />
          <Route path="/monitors" element={<MachineManage />} />
          <Route path="/widgets" element={<HawkeyeWidgets />} />
          <Route path="/alerts" element={<AlertConfig />} />
        </Route>
      </Routes>
    </Suspense>
  )
}
