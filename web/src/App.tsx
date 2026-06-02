import { useState, useCallback } from 'react'
import { Routes, Route } from 'react-router-dom'
import Layout from './components/Layout'
import Dashboard from './pages/Dashboard'
import MachineDetail from './pages/MachineDetail'
import MachineManage from './pages/MachineManage'
import HostStatus from './pages/HostStatus'
import ServiceStatus from './pages/ServiceStatus'
import ProbeDetail from './pages/ProbeDetail'
import AlertConfig from './pages/AlertConfig'
import HomepageWidgets from './pages/HomepageWidgets'
import LoginPage from './pages/LoginPage'

export default function App() {
  const [loggedIn, setLoggedIn] = useState(() => !!localStorage.getItem('auth_token'))

  const handleLogin = useCallback(() => setLoggedIn(true), [])

  if (!loggedIn) {
    return <LoginPage onLogin={handleLogin} />
  }

  return (
    <Routes>
      <Route element={<Layout />}>
        <Route path="/" element={<Dashboard />} />
        <Route path="/machine/:name" element={<MachineDetail />} />
        <Route path="/hosts" element={<HostStatus />} />
        <Route path="/services" element={<ServiceStatus />} />
        <Route path="/probe/:id" element={<ProbeDetail />} />
        <Route path="/monitors" element={<MachineManage />} />
        <Route path="/widgets" element={<HomepageWidgets />} />
        <Route path="/alerts" element={<AlertConfig />} />
      </Route>
    </Routes>
  )
}
