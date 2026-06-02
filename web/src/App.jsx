import { useState, useEffect } from 'react'
import { Routes, Route, NavLink } from 'react-router-dom'
import PatchSummary from './pages/PatchSummary'
import Settings from './pages/Settings'
import SetupWizard from './pages/SetupWizard'
import { FileText, Settings as SettingsIcon, Shield, Cpu } from 'lucide-react'
import { setupApi } from './api'

export default function App() {
  const [setupStatus, setSetupStatus] = useState(null)
  const [loadingSetup, setLoadingSetup] = useState(true)

  useEffect(() => {
    setupApi.getStatus().then(res => {
      setSetupStatus(res.data)
      setLoadingSetup(false)
    }).catch(() => {
      setLoadingSetup(false)
    })
  }, [])

  const handleSetupComplete = async () => {
    const res = await setupApi.getStatus()
    setSetupStatus(res.data)
  }

  if (loadingSetup) {
    return (
      <div className="loading-setup">
        <div className="loading-setup-inner">
          <div className="logo-icon-large">
            <Shield size={28} />
          </div>
          <div className="loading-spinner" style={{ width: 32, height: 32, borderWidth: 3, marginTop: 20 }} />
          <p style={{ marginTop: 12, color: 'var(--text-secondary)', fontSize: 14 }}>正在初始化...</p>
        </div>
      </div>
    )
  }

  const needsSetup = setupStatus && !setupStatus.setup_completed && !setupStatus.has_accounts && !setupStatus.has_jira_config

  if (needsSetup) {
    return <SetupWizard onComplete={handleSetupComplete} />
  }

  return (
    <div className="app-layout">
      <aside className="sidebar">
        <div className="sidebar-brand">
          <div className="logo-icon">
            <Shield size={22} />
          </div>
          <div className="brand-text">
            <h1>Patch 助手</h1>
            <span>智能 Patch 分析平台</span>
          </div>
        </div>

        <div className="sidebar-section">
          <div className="sidebar-section-label">核心功能</div>
          <nav className="sidebar-nav">
            <NavLink to="/" className={({ isActive }) => `nav-item ${isActive ? 'active' : ''}`} end>
              <FileText size={18} /> Patch 汇总
            </NavLink>
          </nav>
        </div>

        <div className="sidebar-section">
          <div className="sidebar-section-label">系统配置</div>
          <nav className="sidebar-nav">
            <NavLink to="/settings" className={({ isActive }) => `nav-item ${isActive ? 'active' : ''}`}>
              <SettingsIcon size={18} /> 设置
            </NavLink>
          </nav>
        </div>

        <div className="sidebar-footer">
          <div className="sidebar-footer-inner">
            <Cpu size={14} />
            <span>Patch Assistant v1.0</span>
          </div>
        </div>
      </aside>
      <main className="main-content">
        <Routes>
          <Route path="/" element={<PatchSummary />} />
          <Route path="/settings" element={<Settings />} />
        </Routes>
      </main>
    </div>
  )
}
