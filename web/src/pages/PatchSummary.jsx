import { useState, useEffect } from 'react'
import { patchApi, accountApi, mailApi, aiApi } from '../api'
import { FileText, RefreshCw, Filter, ChevronDown, ChevronRight, Package, Tag, Calendar, X, ArrowLeft, Settings, Sparkles, Trash2, Star, Plus, Edit3, Shield, Search, Mail } from 'lucide-react'
import dayjs from 'dayjs'
import ReactMarkdown from 'react-markdown'
import remarkGfm from 'remark-gfm'
import rehypeHighlight from 'rehype-highlight'

export default function PatchSummary() {
  const [patches, setPatches] = useState(null)
  const [accounts, setAccounts] = useState([])
  const [selectedAccount, setSelectedAccount] = useState('')
  const [timeRange, setTimeRange] = useState('30d')
  const [customStartDate, setCustomStartDate] = useState('')
  const [customEndDate, setCustomEndDate] = useState('')
  const [loading, setLoading] = useState(false)
  const [syncing, setSyncing] = useState(() => {
    return sessionStorage.getItem('patch_syncing') === 'true'
  })
  const [syncResults, setSyncResults] = useState(() => {
    try { const s = sessionStorage.getItem('patch_syncResults'); return s ? JSON.parse(s) : null } catch { return null }
  })
  const [staleMinutes, setStaleMinutes] = useState(() => {
    const v = sessionStorage.getItem('patch_staleMinutes'); return v ? Number(v) : null
  })
  const [error, setError] = useState('')
  const [expandedProduct, setExpandedProduct] = useState({})

  // 邮件详情面板状态
  const [selectedPatch, setSelectedPatch] = useState(null)
  const [mailDetail, setMailDetail] = useState(null)
  const [loadingDetail, setLoadingDetail] = useState(false)

  // AI 相关状态
  const [showAIConfig, setShowAIConfig] = useState(false)
  const [aiConfigs, setAiConfigs] = useState([])
  const [aiSummarizing, setAiSummarizing] = useState({})
  const [aiResults, setAiResults] = useState({})
  const [showAIPanel, setShowAIPanel] = useState(false)
  const [aiResult, setAiResult] = useState(null)
  const [aiLoading, setAiLoading] = useState(false)

  // Patch 搜索相关状态
  const [searchKeyword, setSearchKeyword] = useState('')
  const [searchResults, setSearchResults] = useState(null)
  const [searching, setSearching] = useState(false)
  const [searchStep, setSearchStep] = useState('') // 搜索提示
  const [searchError, setSearchError] = useState('')

  useEffect(() => { loadAccounts() }, [])

  // 如果从 sessionStorage 恢复了 syncing 状态（用户中途离开又返回），保持同步 UI 并等待完成
  useEffect(() => {
    if (sessionStorage.getItem('patch_syncing') !== 'true') return
    // 同步可能仍在后台进行，轮询 sessionStorage 等待 syncResults 出现
    const poll = setInterval(() => {
      if (sessionStorage.getItem('patch_syncResults')) {
        // 同步已完成，加载结果
        clearInterval(poll)
        setSyncing(false)
        sessionStorage.removeItem('patch_syncing')
        loadSummary(false)
      }
    }, 2000)
    // 5 分钟超时保护
    const timeout = setTimeout(() => {
      clearInterval(poll)
      setSyncing(false)
      sessionStorage.removeItem('patch_syncing')
      loadSummary(false)
    }, 300000)
    return () => { clearInterval(poll); clearTimeout(timeout) }
  }, [])

  useEffect(() => { loadSummary(false) }, [selectedAccount, timeRange, customStartDate, customEndDate])

  // 面板打开时锁定背景页面滚动，关闭时恢复
  useEffect(() => {
    const el = document.querySelector('.main-content')
    if (selectedPatch || showAIPanel) {
      if (el) el.style.overflow = 'hidden'
      document.body.style.overflow = 'hidden'
    } else {
      if (el) el.style.overflow = ''
      document.body.style.overflow = ''
    }
    return () => {
      if (el) el.style.overflow = ''
      document.body.style.overflow = ''
    }
  }, [selectedPatch, showAIPanel])

  // 定时检查同步状态，超过5分钟提示用户刷新
  useEffect(() => {
    const checkSyncStatus = async () => {
      try {
        const res = await patchApi.syncStatus()
        const statuses = res.data || []
        if (statuses.length === 0) return
        // 找最早的同步时间
        let earliest = null
        for (const s of statuses) {
          if (!s.last_sync_at) { earliest = null; break }
          const t = new Date(s.last_sync_at)
          if (!earliest || t < earliest) earliest = t
        }
        let mins = null
        if (earliest) {
          const diffMin = Math.floor((Date.now() - earliest.getTime()) / 60000)
          if (diffMin >= 5) mins = diffMin
        } else {
          mins = 999 // 从未同步
        }
        setStaleMinutes(mins)
        if (mins !== null) sessionStorage.setItem('patch_staleMinutes', String(mins))
        else sessionStorage.removeItem('patch_staleMinutes')
      } catch (e) { /* 静默失败 */ }
    }
    checkSyncStatus()
    const timer = setInterval(checkSyncStatus, 60000) // 每分钟检查
    return () => clearInterval(timer)
  }, [])

  const loadAccounts = async () => {
    try {
      const res = await accountApi.list()
      setAccounts(res.data || [])
    } catch (e) { console.error('加载账户失败', e) }
  }

  const loadAIConfigs = async () => {
    try {
      const res = await aiApi.listConfigs()
      setAiConfigs(res.data || [])
    } catch (e) { console.error('加载AI配置失败', e) }
  }

  const loadSummary = async (withSync = false) => {
    setLoading(true)
    setError('')
    // 切换时间范围/账户时，旧的同步结果已不匹配新范围，必须清除
    setSyncResults(null)
    sessionStorage.removeItem('patch_syncResults')
    try {
      const params = { range: timeRange }
      if (withSync) params.sync = 'true'
      if (selectedAccount) params.account_id = selectedAccount
      if (timeRange === 'custom') {
        if (customStartDate) params.start_date = customStartDate
        if (customEndDate) params.end_date = customEndDate
      }
      const res = await patchApi.summary(params)
      setPatches(res.data)
      if (res.data?.sync_results) {
        setSyncResults(res.data.sync_results)
        sessionStorage.setItem('patch_syncResults', JSON.stringify(res.data.sync_results))
        // 同步完成，清除 syncing 标记
        sessionStorage.removeItem('patch_syncing')
      }
      // 加载完成后，批量获取已有 AI 缓存
      loadAICache(res.data?.patches || [])
    } catch (e) {
      setError(e.message || '加载失败')
      // 同步请求失败时清除 syncing 标记，避免用户回来卡在同步状态
      if (withSync) sessionStorage.removeItem('patch_syncing')
    } finally { setLoading(false) }
  }

  const loadAICache = async (patchList) => {
    if (!patchList || patchList.length === 0) return
    const mailIds = patchList.map(p => p.mail_id)
    try {
      const res = await aiApi.batchSummaries(mailIds)
      const cachedMap = res.data || {}
      // 将后端返回的 map 转为前端 aiResults 格式
      const newResults = {}
      for (const [mailId, summary] of Object.entries(cachedMap)) {
        if (summary) {
          newResults[Number(mailId)] = summary
        }
      }
      setAiResults(prev => ({ ...prev, ...newResults }))
    } catch (e) {
      console.error('加载AI缓存失败', e)
    }
  }

  const handleSyncAndRefresh = async () => {
    setSyncing(true)
    sessionStorage.setItem('patch_syncing', 'true')
    setStaleMinutes(null)
    setSyncResults(null)
    sessionStorage.removeItem('patch_staleMinutes')
    sessionStorage.removeItem('patch_syncResults')

    // 直接调同步（不再先调 imap-count 预计数，避免重复 IMAP 连接导致 2x 耗时）
    try { await loadSummary(true) } finally {
      setSyncing(false)
    }
  }

  const handleViewDetail = async (patch) => {
    setSelectedPatch(patch)
    setMailDetail(null)
    setLoadingDetail(true)
    try {
      const res = await mailApi.detail(patch.mail_id)
      setMailDetail(res.data || null)
    } catch (err) {
      setMailDetail(null)
    } finally { setLoadingDetail(false) }
  }

  const handleCloseDetail = () => { setSelectedPatch(null); setMailDetail(null) }
  const handleCloseAIPanel = () => { setShowAIPanel(false) }

  // Patch 搜索
  async function handleSearch() {
    const kw = searchKeyword.trim()
    if (!kw || searching) return
    setSearching(true)
    setSearchResults(null)
    setSearchStep('')
    setSearchError('')
    try {
      const res = await patchApi.search(kw, selectedAccount || 0)
      const data = res.data
      setSearchResults(data.patches || [])
      if ((data.patches || []).length === 0) {
        setSearchStep('未找到匹配的 Patch')
      } else {
        setSearchStep('')
      }
    } catch (err) {
      setSearchError(err.message || '搜索失败')
      setSearchStep('')
    } finally {
      setSearching(false)
    }
  }

  function clearSearch() {
    setSearchKeyword('')
    setSearchResults(null)
    setSearchStep('')
    setSearchError('')
  }

  const handleAISummarize = async (patch, force = false) => {
    // 已有缓存且非强制刷新，直接展示
    if (!force) {
      const cached = aiResults[patch.mail_id]
      if (cached && !cached.error) {
        setAiResult(cached)
        setShowAIPanel(true)
        return
      }
    }

    setAiSummarizing(prev => ({ ...prev, [patch.mail_id]: true }))
    setAiResult(null)
    setShowAIPanel(true)
    setAiLoading(true)
    try {
      const res = await aiApi.summarize(patch.mail_id, { force })
      const result = res.data
      setAiResults(prev => ({ ...prev, [patch.mail_id]: result }))
      setAiResult(result)
    } catch (err) {
      const errMsg = err.message || 'AI 汇总失败'
      const errorResult = { error: errMsg, mail_id: patch.mail_id, subject: patch.subject }
      setAiResults(prev => ({ ...prev, [patch.mail_id]: errorResult }))
      setAiResult(errorResult)
    } finally {
      setAiSummarizing(prev => ({ ...prev, [patch.mail_id]: false }))
      setAiLoading(false)
    }
  }

  const toggleProduct = (product) => {
    setExpandedProduct(prev => ({ ...prev, [product]: !prev[product] }))
  }

  const groupByProduct = (patchList) => {
    if (!patchList) return {}
    const groups = {}
    patchList.forEach(p => {
      const key = p.product || '未知产品'
      if (!groups[key]) groups[key] = []
      groups[key].push(p)
    })
    return groups
  }

  const sortByVersion = (list) => {
    return [...list].sort((a, b) => {
      if (a.version && b.version) return b.version.localeCompare(a.version, undefined, { numeric: true })
      return 0
    })
  }

  const grouped = patches ? groupByProduct(patches.patches) : {}
  const sortedProducts = Object.keys(grouped).sort()

  const typeColorMap = {
    '预览': { bg: '#EFF6FF', text: '#2563EB', border: '#BFDBFE' },
    '通用': { bg: '#ECFDF5', text: '#059669', border: '#A7F3D0' },
    '定向': { bg: '#FFF7ED', text: '#C2410C', border: '#FED7AA' },
  }

  function getInitial(name) {
    if (!name) return '?'
    return name.charAt(0).toUpperCase()
  }

  return (
    <div>
      {/* 筛选栏 */}
      <div className="filter-bar">
        <div className="filter-bar-inner">
          <div className="filter-label">
            <Filter size={16} color="var(--primary)" /> 筛选
          </div>

          <div className="filter-group">
            <label>邮箱账户:</label>
            <select value={selectedAccount} onChange={e => setSelectedAccount(e.target.value)} className="filter-select">
              <option value="">全部账户</option>
              {accounts.map(acc => <option key={acc.id} value={acc.id}>{acc.email}</option>)}
            </select>
          </div>

          <div className="filter-group">
            <label>时间范围:</label>
            <div className="time-range-group">
              {[
                { key: '7d', label: '近7天' },
                { key: '30d', label: '近30天' },
                { key: '90d', label: '近90天' },
                { key: 'week', label: '本周' },
                { key: 'year', label: '本年' },
                { key: 'custom', label: '自定义' },
              ].map(r => (
                <button key={r.key} onClick={() => setTimeRange(r.key)}
                  className={`time-range-btn ${timeRange === r.key ? 'active' : ''}`}>
                  {r.label}
                </button>
              ))}
            </div>
          </div>

          {timeRange === 'custom' && (
            <div className="filter-group">
              <input type="date" value={customStartDate} onChange={e => setCustomStartDate(e.target.value)}
                style={{ padding: '5px 10px', border: '1px solid var(--border)', borderRadius: 'var(--radius)', fontSize: 13, color: 'var(--text)', outline: 'none' }} />
              <span style={{ color: 'var(--text-secondary)', fontSize: 13 }}>至</span>
              <input type="date" value={customEndDate} onChange={e => setCustomEndDate(e.target.value)}
                style={{ padding: '5px 10px', border: '1px solid var(--border)', borderRadius: 'var(--radius)', fontSize: 13, color: 'var(--text)', outline: 'none' }} />
            </div>
          )}

          <button onClick={handleSyncAndRefresh} disabled={syncing} className="btn btn-primary btn-sm" style={{ marginLeft: 'auto' }}>
            <RefreshCw size={14} className={syncing ? 'spin' : ''} />
            {syncing ? '同步中...' : '同步刷新'}
          </button>
        </div>
      </div>

      {/* Patch 搜索栏 */}
      <div className="card" style={{ marginBottom: 16, padding: '12px 16px' }}>
        <div style={{ display: 'flex', gap: 12, alignItems: 'center', flexWrap: 'wrap' }}>
          <Search size={16} color="var(--primary)" style={{ flexShrink: 0 }} />
          <input
            type="text"
            placeholder="输入 WARP 编号、Patch 编号或关键词搜索...（本地未找到自动从服务器同步）"
            value={searchKeyword}
            onChange={e => setSearchKeyword(e.target.value)}
            onKeyDown={e => e.key === 'Enter' && handleSearch()}
            style={{ flex: 1, minWidth: 200, padding: '8px 12px', border: '1px solid var(--border)', borderRadius: 'var(--radius)', fontSize: 14, outline: 'none' }}
          />
          <button onClick={handleSearch} disabled={searching} className="btn btn-primary btn-sm" style={{ flexShrink: 0 }}>
            {searching ? (
              <><div className="loading-spinner" style={{ width: 14, height: 14, borderWidth: 2, display: 'inline-block', verticalAlign: 'middle', marginRight: 4 }} /> 搜索中...</>
            ) : '搜索'}
          </button>
          {searchResults && (
            <button onClick={clearSearch} className="btn btn-secondary btn-sm" style={{ flexShrink: 0 }}>清除</button>
          )}
        </div>
        {searchStep && !searching && (
          <div style={{ marginTop: 8, padding: '6px 12px', background: '#EFF6FF', border: '1px solid #BFDBFE', borderRadius: 'var(--radius)', fontSize: 12, color: '#2563EB' }}>
            {searchStep}
          </div>
        )}
        {searching && searchStep && (
          <div style={{ marginTop: 8, padding: '6px 12px', background: '#EFF6FF', border: '1px solid #BFDBFE', borderRadius: 'var(--radius)', fontSize: 12, color: '#2563EB', display: 'flex', alignItems: 'center', gap: 6 }}>
            <div className="loading-spinner" style={{ width: 12, height: 12, borderWidth: 2, display: 'inline-block' }} />
            {searchStep}
          </div>
        )}
        {searchError && (
          <div style={{ marginTop: 8, padding: '6px 12px', background: '#FEF2F2', border: '1px solid #FECACA', borderRadius: 'var(--radius)', fontSize: 12, color: '#DC2626' }}>
            {searchError}
          </div>
        )}
      </div>

      {/* 数据概览 - 简洁版 */}
      {!searchResults && patches && !loading && !syncing && (
        <div style={{ marginBottom: 16, fontSize: 13, color: 'var(--text-secondary)', display: 'flex', alignItems: 'center', gap: 8 }}>
          <span>当前范围共 <b style={{ color: 'var(--text)' }}>{patches.total_count}</b> 条 Patch</span>
          {accounts.length > 0 && accounts.some(a => a.last_sync_at) && (
            <span style={{ marginLeft: 12 }}>· 上次同步：{accounts.filter(a => a.last_sync_at).sort((a,b) => new Date(b.last_sync_at) - new Date(a.last_sync_at))[0]?.last_sync_at?.replace('T', ' ')?.slice(0, 16) || '-'}</span>
          )}
        </div>
      )}

      {loading && !patches && (
        <div style={{ textAlign: 'center', padding: 40 }}>
          <div className="loading-spinner" style={{ width: 32, height: 32, borderWidth: 3 }} />
          <p style={{ marginTop: 12, color: 'var(--text-secondary)', fontSize: 14 }}>正在加载汇总数据...</p>
        </div>
      )}

      {/* 同步中提示 */}
      {syncing && (
        <div style={{ marginBottom: 16, padding: '10px 16px', background: '#EFF6FF', border: '1px solid #BFDBFE', borderRadius: 'var(--radius-lg)', fontSize: 13, display: 'flex', alignItems: 'center', gap: 8 }}>
          <div className="loading-spinner" style={{ width: 16, height: 16, borderWidth: 2 }} />
          <span style={{ color: '#2563EB' }}>正在同步邮件，请稍候...</span>
        </div>
      )}

      {/* 同步完成提示 */}
      {!syncing && syncResults && syncResults.length > 0 && (() => {
        const totalNewPatch = syncResults.reduce((s, r) => s + (r.new_patch_mails || 0), 0)
        const rangeLabel = {'7d':'近7天','30d':'近30天','90d':'近90天','week':'本周','year':'本年','custom':'选定范围'}[timeRange] || '当前范围'
        return (
          <div style={{ marginBottom: 16, padding: '10px 16px', background: '#F0FDF4', border: '1px solid #BBF7D0', borderRadius: 'var(--radius-lg)', fontSize: 13, display: 'flex', flexDirection: 'column', gap: 4 }}>
            <div style={{ color: '#16A34A', fontWeight: 500, display: 'flex', alignItems: 'center', gap: 6 }}>
              <svg width="16" height="16" viewBox="0 0 16 16" fill="none"><path d="M6.5 12L2.5 8l1.41-1.41L6.5 9.17l5.59-5.59L13.5 5l-7 7z" fill="#16A34A"/></svg>
              同步完成 · {rangeLabel}共 {syncResults[0]?.range_patch_total || 0} 条 Patch
            </div>
            {syncResults.map((sr, i) => (
              <div key={i} style={{ color: '#15803D', paddingLeft: 22 }}>
                {sr.account_email ? `${sr.account_email}：` : ''}
                {sr.error ? (
                  <span style={{ color: '#DC2626' }}>同步失败 - {sr.error}</span>
                ) : (
                  <>
                    {sr.new_patch_mails > 0 ? (
                      <span>新增 <b>{sr.new_patch_mails}</b> 封 Patch 通知</span>
                    ) : (
                      '无新 Patch'
                    )}
                  </>
                )}
              </div>
            ))}
          </div>
        )
      })()}

      {!syncing && staleMinutes && !syncResults && (
        <div style={{ marginBottom: 16, padding: '10px 16px', background: '#FFFBEB', border: '1px solid #FDE68A', borderRadius: 'var(--radius-lg)', fontSize: 13, display: 'flex', alignItems: 'center', gap: 8, cursor: 'pointer' }} onClick={handleSyncAndRefresh}>
          <svg width="16" height="16" viewBox="0 0 16 16" fill="none"><path d="M8 1a7 7 0 100 14A7 7 0 008 1zm.75 4.25v3.5h-1.5v-3.5h1.5zM8 10.5a.75.75 0 110 1.5.75.75 0 010-1.5z" fill="#D97706"/></svg>
          <span style={{ color: '#92400E' }}>
            {staleMinutes >= 999 ? '尚未同步过邮件，点击同步刷新拉取 Patch 通知' :
             `距上次同步已过 ${staleMinutes >= 60 ? Math.floor(staleMinutes/60) + ' 小时 ' + (staleMinutes%60) + ' 分钟' : staleMinutes + ' 分钟'}，可能有新 Patch，点击刷新`}
          </span>
        </div>
      )}

      {!searchResults && error && (
        <div className="card" style={{ borderColor: '#FCA5A5', background: '#FEF2F2' }}>
          <p style={{ color: '#DC2626', fontSize: 14 }}>{error}</p>
        </div>
      )}

      {/* 搜索结果展示 */}
      {searchResults && (
        <div style={{ marginBottom: 16 }}>
          <div style={{ marginBottom: 12, fontSize: 13, color: 'var(--text-secondary)', display: 'flex', alignItems: 'center', gap: 8 }}>
            <Search size={14} />
            搜索 "<b style={{ color: 'var(--text)' }}>{searchKeyword}</b>" 找到 <b style={{ color: 'var(--text)' }}>{searchResults.length}</b> 条 Patch
          </div>
          {searchResults.length > 0 ? (
            <div className="card" style={{ padding: 0, overflow: 'hidden' }}>
              <div className="table-wrapper">
                <table style={{ tableLayout: 'fixed', width: '100%' }}>
                  <colgroup>
                    <col style={{ width: 70 }} />
                    <col style={{ width: 260 }} />
                    <col style={{ width: 120 }} />
                    <col style={{ width: 100 }} />
                    <col style={{ width: 130 }} />
                    <col style={{ width: 220 }} />
                  </colgroup>
                  <thead>
                    <tr><th>类型</th><th>Patch 名称</th><th>产品</th><th>版本</th><th>Patch 日期</th><th>操作</th></tr>
                  </thead>
                  <tbody>
                    {searchResults.map((p, idx) => {
                      const tc = typeColorMap[p.type] || typeColorMap['通用']
                      const patchName = p.product && p.version && p.patch_date
                        ? `Patch-${p.product}-${p.version}-${p.patch_date}`
                        : (p.subject || '-')
                      return (
                        <tr key={p.mail_id || idx}>
                          <td><span style={{ display: 'inline-flex', alignItems: 'center', padding: '2px 8px', borderRadius: 9999, fontSize: 12, fontWeight: 500, background: tc.bg, color: tc.text, border: `1px solid ${tc.border}` }}>{p.type || '通用'}</span></td>
                          <td style={{ fontWeight: 500, overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }} title={patchName}>{patchName}</td>
                          <td style={{ fontWeight: 500, overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}>{p.product || '-'}</td>
                          <td style={{ fontFamily: 'monospace', fontSize: 13, overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}>{p.version || '-'}</td>
                          <td style={{ overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}>{p.patch_date ? (<span style={{ display: 'inline-flex', alignItems: 'center', gap: 4 }}><Calendar size={12} />{p.patch_date.slice(0, 4)}-{p.patch_date.slice(4, 6)}-{p.patch_date.slice(6, 8)}</span>) : '-'}</td>
                          <td style={{ whiteSpace: 'nowrap' }}>
                            <div style={{ display: 'flex', gap: 6 }}>
                              <ActionBtn label="详情" color="var(--primary)" onClick={() => handleViewDetail(p)} />
                              <ActionBtn
                                label="AI 分析"
                                color="#8B5CF6"
                                icon={<Sparkles size={12} />}
                                onClick={() => handleAISummarize(p)}
                              />
                            </div>
                          </td>
                        </tr>
                      )
                    })}
                  </tbody>
                </table>
              </div>
            </div>
          ) : (
            <div className="card" style={{ padding: 32, textAlign: 'center', color: 'var(--text-muted)' }}>
              <Mail size={36} style={{ marginBottom: 8 }} />
              <p>未找到匹配的 Patch</p>
              <p style={{ fontSize: 12 }}>请确认关键词是否正确，或尝试从邮件服务器同步</p>
            </div>
          )}
        </div>
      )}

      {!searchResults && !loading && !error && patches && (
        <>
          <div className="stats-grid">
            <div className="stat-card">
              <div className="stat-icon" style={{ background: 'var(--gradient-primary-light)' }}><FileText size={20} color="var(--primary)" /></div>
              <div className="stat-value">{patches.total_count}</div>
              <div className="stat-label">Patch 总数</div>
            </div>
            <div className="stat-card">
              <div className="stat-icon" style={{ background: '#ECFDF5' }}><Package size={20} color="var(--success)" /></div>
              <div className="stat-value">{Object.keys(patches.by_product || {}).length}</div>
              <div className="stat-label">涉及产品</div>
            </div>
            <div className="stat-card">
              <div className="stat-icon" style={{ background: '#EFF6FF' }}><Tag size={20} color="var(--info)" /></div>
              <div className="stat-value">{patches.by_type?.['预览'] || 0}</div>
              <div className="stat-label">预览 Patch</div>
            </div>
            <div className="stat-card">
              <div className="stat-icon" style={{ background: '#ECFDF5' }}><Tag size={20} color="var(--success)" /></div>
              <div className="stat-value">{patches.by_type?.['通用'] || 0}</div>
              <div className="stat-label">通用 Patch</div>
            </div>
          </div>

          {sortedProducts.length === 0 ? (
            <div className="card">
              <div className="empty-state">
                <Shield size={48} />
                <p>未找到 Patch 相关邮件</p>
                <p style={{ fontSize: 12, color: 'var(--text-muted)' }}>点击"同步刷新"拉取最新邮件</p>
              </div>
            </div>
          ) : sortedProducts.map(product => {
            const items = sortByVersion(grouped[product])
            const isExpanded = expandedProduct[product] !== false
            return (
              <div key={product} className="product-card">
                <div className="product-card-header" onClick={() => toggleProduct(product)}>
                  <div className="product-card-header-left">
                    {isExpanded ? <ChevronDown size={18} /> : <ChevronRight size={18} />}
                    <Package size={18} color="var(--primary)" />
                    <h3 style={{ fontSize: 16, fontWeight: 600 }}>{product}</h3>
                    <span className="product-badge">{items.length} 个 Patch</span>
                  </div>
                </div>
                {isExpanded && (
                  <div className="product-card-body">
                    <div className="table-wrapper">
                      <table style={{ tableLayout: 'fixed', width: '100%' }}>
                        <colgroup>
                          <col style={{ width: 70 }} />
                          <col style={{ width: 260 }} />
                          <col style={{ width: 120 }} />
                          <col style={{ width: 100 }} />
                          <col style={{ width: 130 }} />
                          <col style={{ width: 220 }} />
                        </colgroup>
                        <thead>
                          <tr><th>类型</th><th>Patch 名称</th><th>产品</th><th>版本</th><th>Patch 日期</th><th>操作</th></tr>
                        </thead>
                        <tbody>
                          {items.map((p, idx) => {
                            const tc = typeColorMap[p.type] || typeColorMap['通用']
                            const isAIWorking = aiSummarizing[p.mail_id]
                            const hasAIResult = aiResults[p.mail_id]
                            // 构造简短的 Patch 名称：Patch-产品-版本-日期
                            const patchName = p.product && p.version && p.patch_date
                              ? `Patch-${p.product}-${p.version}-${p.patch_date}`
                              : (p.subject || '-')
                            return (
                              <tr key={idx}>
                                <td><span style={{ display: 'inline-flex', alignItems: 'center', padding: '2px 8px', borderRadius: 9999, fontSize: 12, fontWeight: 500, background: tc.bg, color: tc.text, border: `1px solid ${tc.border}` }}>{p.type || '通用'}</span></td>
                                <td style={{ fontWeight: 500, overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }} title={patchName}>{patchName}</td>
                                <td style={{ fontWeight: 500, overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}>{p.product || '-'}</td>
                                <td style={{ fontFamily: 'monospace', fontSize: 13, overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}>{p.version || '-'}</td>
                                <td style={{ overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}>{p.patch_date ? (<span style={{ display: 'inline-flex', alignItems: 'center', gap: 4 }}><Calendar size={12} />{p.patch_date.slice(0, 4)}-{p.patch_date.slice(4, 6)}-{p.patch_date.slice(6, 8)}</span>) : '-'}</td>
                                <td style={{ whiteSpace: 'nowrap' }}>
                                  <div style={{ display: 'flex', gap: 6 }}>
                                    <ActionBtn label="详情" color="var(--primary)" onClick={() => handleViewDetail(p)} />
                                    <ActionBtn
                                      label={isAIWorking ? '分析中' : (hasAIResult && !hasAIResult.error ? '查看分析' : 'AI 分析')}
                                      color="#8B5CF6"
                                      icon={<Sparkles size={12} className={isAIWorking ? 'spin' : ''} />}
                                      disabled={isAIWorking}
                                      active={hasAIResult && !hasAIResult.error}
                                      onClick={() => handleAISummarize(p)}
                                    />
                                    {hasAIResult && !hasAIResult.error && !isAIWorking && (
                                      <ActionBtn
                                        label="重新分析"
                                        color="#F59E0B"
                                        icon={<RefreshCw size={12} />}
                                        onClick={() => handleAISummarize(p, true)}
                                      />
                                    )}
                                  </div>
                                </td>
                              </tr>
                            )
                          })}
                        </tbody>
                      </table>
                    </div>
                  </div>
                )}
              </div>
            )
          })}
        </>
      )}

      {/* 邮件详情面板 */}
      {selectedPatch && <><div onClick={handleCloseDetail} style={{ position: 'fixed', top: 0, left: 0, right: 0, bottom: 0, backgroundColor: 'rgba(0,0,0,0.3)', zIndex: 999 }} />
        <div style={{ position: 'fixed', top: 0, right: 0, bottom: 0, width: '60%', minWidth: 480, backgroundColor: '#FFFFFF', borderLeft: '1px solid var(--border)', boxShadow: '-4px 0 24px rgba(0,0,0,0.12)', zIndex: 1000, display: 'flex', flexDirection: 'column', overflow: 'hidden' }}>
          <div style={{ padding: '16px 24px', borderBottom: '1px solid var(--border)', display: 'flex', alignItems: 'center', gap: 12, backgroundColor: '#F8FAFC' }}>
            <button onClick={handleCloseDetail} style={{ display: 'flex', alignItems: 'center', gap: 4, padding: '6px 12px', border: '1px solid var(--border)', borderRadius: 'var(--radius)', background: '#FFFFFF', cursor: 'pointer', fontSize: 13, color: 'var(--text-secondary)' }}><ArrowLeft size={14} /> 返回</button>
            <span style={{ flex: 1 }}></span>
            <button onClick={handleCloseDetail} style={{ padding: 4, border: 'none', background: 'none', cursor: 'pointer', color: 'var(--text-muted)' }}><X size={18} /></button>
          </div>
          <div style={{ padding: '20px 24px', borderBottom: '1px solid var(--border)' }}>
            <h2 style={{ margin: '0 0 12px 0', fontSize: 18, fontWeight: 600, color: 'var(--text)' }}>{selectedPatch.subject || '(无主题)'}</h2>
            <div style={{ display: 'flex', alignItems: 'center', gap: 12, marginBottom: 8 }}>
              <div style={{ width: 36, height: 36, borderRadius: '50%', background: 'var(--gradient-primary)', color: '#fff', display: 'flex', alignItems: 'center', justifyContent: 'center', fontSize: 14, fontWeight: 600, flexShrink: 0 }}>{getInitial(selectedPatch.from_name || selectedPatch.from_addr)}</div>
              <div>
                <div style={{ fontWeight: 500, fontSize: 14 }}>{selectedPatch.from_name || selectedPatch.from_addr}</div>
                <div style={{ fontSize: 12, color: 'var(--text-muted)' }}>&lt;{selectedPatch.from_addr}&gt;</div>
              </div>
              <span style={{ marginLeft: 'auto', fontSize: 12, color: 'var(--text-muted)' }}>{dayjs(selectedPatch.mail_date).format('YYYY-MM-DD HH:mm')}</span>
            </div>
          </div>
          <div style={{ flex: 1, overflow: 'auto', padding: '20px 24px' }}>
            {loadingDetail ? (<div style={{ textAlign: 'center', padding: 48 }}><div className="loading-spinner" /><p style={{ color: 'var(--text-muted)', marginTop: 12 }}>加载邮件内容...</p></div>)
              : mailDetail ? (<MailBodyContent detail={mailDetail} />)
              : (<div style={{ textAlign: 'center', padding: 48, color: 'var(--text-muted)' }}><FileText size={36} /><p style={{ marginTop: 12 }}>无法加载邮件内容</p><p style={{ fontSize: 12 }}>请同步刷新邮件后再试</p></div>)}
          </div>
        </div>
      </>}

      {/* AI 汇总面板 */}
      {showAIPanel && <><div onClick={handleCloseAIPanel} style={{ position: 'fixed', top: 0, left: 0, right: 0, bottom: 0, backgroundColor: 'rgba(0,0,0,0.3)', zIndex: 1099 }} />
        <div style={{ position: 'fixed', top: 0, right: 0, bottom: 0, width: '55%', minWidth: 460, backgroundColor: '#FFFFFF', borderLeft: '1px solid var(--border)', boxShadow: '-4px 0 24px rgba(0,0,0,0.12)', zIndex: 1100, display: 'flex', flexDirection: 'column', overflow: 'hidden' }}>
          <div style={{ padding: '16px 24px', borderBottom: '1px solid var(--border)', display: 'flex', alignItems: 'center', gap: 12, background: 'linear-gradient(135deg, #F5F3FF 0%, #EEF2FF 100%)' }}>
            <Sparkles size={18} color="#7C3AED" />
            <h3 style={{ margin: 0, fontSize: 16, fontWeight: 600, color: '#5B21B6' }}>AI Patch 分析</h3>
            {aiResult && !aiResult.error && !aiLoading && (
              <button
                onClick={() => { if (selectedPatch || aiResult) handleAISummarize({ mail_id: aiResult.mail_id, subject: aiResult.subject }, true) }}
                style={{ marginLeft: 8, display: 'inline-flex', alignItems: 'center', gap: 4, padding: '4px 10px', borderRadius: 'var(--radius)', fontSize: 12, fontWeight: 500, border: '1px solid #F59E0B', background: 'transparent', color: '#B45309', cursor: 'pointer', transition: 'all 0.15s' }}
                onMouseEnter={e => { e.target.style.background = '#F59E0B'; e.target.style.color = '#fff' }}
                onMouseLeave={e => { e.target.style.background = 'transparent'; e.target.style.color = '#B45309' }}
              >
                <RefreshCw size={12} /> 重新分析
              </button>
            )}
            <span style={{ flex: 1 }}></span>
            <button onClick={handleCloseAIPanel} style={{ padding: 4, border: 'none', background: 'none', cursor: 'pointer', color: 'var(--text-muted)' }}><X size={18} /></button>
          </div>
          <div style={{ flex: 1, overflow: 'auto', padding: '20px 24px' }}>
            {aiLoading ? (
              <div style={{ textAlign: 'center', padding: 48 }}>
                <div className="loading-spinner" style={{ width: 32, height: 32, borderWidth: 3 }} />
                <p style={{ color: '#7C3AED', marginTop: 16, fontWeight: 500 }}>AI 正在分析邮件内容...</p>
                <p style={{ color: 'var(--text-muted)', fontSize: 13, marginTop: 8 }}>这可能需要几秒到几十秒</p>
              </div>
            ) : aiResult ? (
              aiResult.error ? (
                <div style={{ padding: 20, background: '#FEF2F2', border: '1px solid #FECACA', borderRadius: 'var(--radius-lg)', color: '#DC2626' }}>
                  <p style={{ fontWeight: 600, marginBottom: 8 }}>AI 汇总失败</p>
                  <p style={{ fontSize: 13 }}>{aiResult.error}</p>
                  <p style={{ fontSize: 12, color: 'var(--text-muted)', marginTop: 12 }}>请检查 AI 配置是否正确（点击页面顶部"AI 配置"按钮）</p>
                </div>
              ) : (
                <div>
                  <div style={{ padding: '12px 16px', background: 'linear-gradient(135deg, #F5F3FF 0%, #EEF2FF 100%)', borderRadius: 'var(--radius-lg)', marginBottom: 16, borderLeft: '3px solid #7C3AED' }}>
                    <p style={{ margin: 0, fontSize: 13, color: '#5B21B6', fontWeight: 500 }}>{aiResult.subject}</p>
                    <p style={{ margin: '4px 0 0', fontSize: 12, color: '#8B5CF6' }}>使用 {aiResult.provider} / {aiResult.model}</p>
                    {aiResult.created_at && <p style={{ margin: '2px 0 0', fontSize: 11, color: '#A78BFA' }}>分析时间：{new Date(aiResult.created_at).toLocaleString('zh-CN')}</p>}
                  </div>
                  {aiResult.jira_links && aiResult.jira_links.length > 0 && (
                    <div style={{ padding: '10px 16px', background: '#FFFBEB', borderRadius: 'var(--radius-lg)', marginBottom: 16, borderLeft: '3px solid #F59E0B' }}>
                      <p style={{ margin: '0 0 6px', fontSize: 13, fontWeight: 600, color: '#92400E' }}>🔗 JIRA 工单</p>
                      <div style={{ display: 'flex', flexWrap: 'wrap', gap: 6 }}>
                        {aiResult.jira_links.map((link, i) => (
                          <a key={i} href={link.url} target="_blank" rel="noopener noreferrer"
                            style={{ display: 'inline-flex', alignItems: 'center', gap: 4, padding: '3px 10px', borderRadius: 'var(--radius)', fontSize: 12, fontWeight: 500, background: '#FEF3C7', color: '#B45309', border: '1px solid #FCD34D', textDecoration: 'none', transition: 'all 0.15s' }}
                            onMouseEnter={e => { e.target.style.background = '#FDE68A' }}
                            onMouseLeave={e => { e.target.style.background = '#FEF3C7' }}>
                            {link.key}
                          </a>
                        ))}
                      </div>
                    </div>
                  )}
                  {aiResult.wiki_links && aiResult.wiki_links.length > 0 && (
                    <div style={{ padding: '10px 16px', background: '#F0FDF4', borderRadius: 'var(--radius-lg)', marginBottom: 16, borderLeft: '3px solid #10B981' }}>
                      <p style={{ margin: '0 0 6px', fontSize: 13, fontWeight: 600, color: '#065F46' }}>📄 Wiki 文档</p>
                      <div style={{ display: 'flex', flexWrap: 'wrap', gap: 6 }}>
                        {aiResult.wiki_links.map((link, i) => (
                          <a key={i} href={link.url} target="_blank" rel="noopener noreferrer"
                            style={{ display: 'inline-flex', alignItems: 'center', gap: 4, padding: '3px 10px', borderRadius: 'var(--radius)', fontSize: 12, fontWeight: 500, background: '#D1FAE5', color: '#047857', border: '1px solid #6EE7B7', textDecoration: 'none', transition: 'all 0.15s' }}
                            onMouseEnter={e => { e.target.style.background = '#A7F3D0' }}
                            onMouseLeave={e => { e.target.style.background = '#D1FAE5' }}>
                            {link.title}
                          </a>
                        ))}
                      </div>
                    </div>
                  )}
                  <MarkdownContent content={aiResult.summary} />
                </div>
              )
            ) : null}
          </div>
        </div>
      </>}

      {/* AI 配置弹窗 */}
      {showAIConfig && <AIConfigModal configs={aiConfigs} onClose={() => setShowAIConfig(false)} onRefresh={loadAIConfigs} />}

      <style>{`.spin { animation: spin 0.8s linear infinite; }`}</style>
    </div>
  )
}

// 操作按钮组件
function ActionBtn({ label, color, icon, disabled, active, onClick }) {
  const [hover, setHover] = useState(false)
  return (
    <button
      onClick={(e) => { e.stopPropagation(); onClick() }}
      disabled={disabled}
      onMouseEnter={() => setHover(true)}
      onMouseLeave={() => setHover(false)}
      style={{
        display: 'inline-flex', alignItems: 'center', gap: 4, padding: '4px 10px',
        borderRadius: 'var(--radius)', fontSize: 12, fontWeight: 500,
        cursor: disabled ? 'not-allowed' : 'pointer',
        border: `1px solid ${color}`,
        background: hover || active ? color : 'transparent',
        color: hover || active ? '#fff' : color,
        opacity: disabled ? 0.6 : 1,
        transition: 'all 0.15s'
      }}
    >
      {icon}{label}
    </button>
  )
}

// Markdown 内容渲染组件
function MarkdownContent({ content }) {
  if (!content) return null

  return (
    <div className="markdown-body" style={{
      background: '#FAFAFA', padding: 20, borderRadius: 'var(--radius-lg)',
      border: '1px solid var(--border)'
    }}>
      <ReactMarkdown
        remarkPlugins={[remarkGfm]}
        rehypePlugins={[rehypeHighlight]}
        components={{
          // 为表格自动添加包裹层，支持横向滚动
          table: ({ children }) => (
            <div className="table-wrapper" style={{ overflowX: 'auto', margin: '12px 0' }}>
              <table>{children}</table>
            </div>
          ),
        }}
      >
        {content}
      </ReactMarkdown>
    </div>
  )
}

// 正文内容渲染组件
function MailBodyContent({ detail }) {
  const [showHtml, setShowHtml] = useState(true)
  const hasHtml = !!detail.body_html
  const hasText = !!detail.body_text

  if (!hasHtml && !hasText) {
    return (
      <div style={{ textAlign: 'center', padding: 48, color: 'var(--text-muted)' }}>
        <p>此邮件没有正文内容</p>
        <p style={{ fontSize: 12 }}>可能是纯附件邮件或同步时未拉取正文</p>
      </div>
    )
  }

  return (
    <div>
      {hasHtml && hasText && (
        <div style={{ marginBottom: 12, display: 'flex', gap: 8 }}>
          <button onClick={() => setShowHtml(true)} style={{ padding: '4px 12px', fontSize: 12, borderRadius: 'var(--radius)', border: '1px solid var(--border)', cursor: 'pointer', background: showHtml ? 'var(--primary)' : '#FFFFFF', color: showHtml ? '#fff' : 'var(--text-secondary)' }}>富文本</button>
          <button onClick={() => setShowHtml(false)} style={{ padding: '4px 12px', fontSize: 12, borderRadius: 'var(--radius)', border: '1px solid var(--border)', cursor: 'pointer', background: !showHtml ? 'var(--primary)' : '#FFFFFF', color: !showHtml ? '#fff' : 'var(--text-secondary)' }}>纯文本</button>
        </div>
      )}
      {showHtml && hasHtml ? (
        <div style={{ border: '1px solid var(--border)', borderRadius: 'var(--radius-lg)', overflow: 'hidden', backgroundColor: '#fff' }}>
          <iframe srcDoc={detail.body_html} title="邮件正文" style={{ width: '100%', minHeight: 400, border: 'none', display: 'block' }} sandbox="allow-same-origin" scrolling="no"
            onLoad={(e) => { const iframe = e.target; try { const doc = iframe.contentDocument; if (doc && doc.body) { iframe.style.height = Math.max(doc.body.scrollHeight + 20, 400) + 'px'; doc.body.style.overflow = 'hidden' } } catch {} }}
          />
        </div>
      ) : (
        <pre style={{ whiteSpace: 'pre-wrap', wordBreak: 'break-word', fontSize: 14, lineHeight: 1.7, color: 'var(--text)', backgroundColor: '#F8FAFC', padding: 16, borderRadius: 'var(--radius-lg)', border: '1px solid var(--border)', margin: 0, fontFamily: 'inherit' }}>{detail.body_text}</pre>
      )}
    </div>
  )
}

// AI 配置弹窗组件
function AIConfigModal({ configs, onClose, onRefresh }) {
  const [editing, setEditing] = useState(null)
  const [form, setForm] = useState({ name: '', endpoint: '', api_key: '', model: '', is_default: false })
  const [saving, setSaving] = useState(false)

  // Prompt 编辑状态
  const [aiPrompt, setAiPrompt] = useState('')
  const [promptLoaded, setPromptLoaded] = useState(false)
  const [promptSaving, setPromptSaving] = useState(false)
  const [promptSaved, setPromptSaved] = useState(false)

  // 加载 prompt
  useEffect(() => {
    if (!promptLoaded) {
      aiApi.getPrompt().then(res => {
        setAiPrompt(res.data?.prompt || '')
        setPromptLoaded(true)
      }).catch(() => setPromptLoaded(true))
    }
  }, [promptLoaded])

  const savePrompt = async () => {
    setPromptSaving(true)
    try {
      await aiApi.savePrompt(aiPrompt)
      setPromptSaved(true)
      setTimeout(() => setPromptSaved(false), 2000)
    } catch (e) { alert('保存提示词失败: ' + (e.message || '未知错误')) }
    finally { setPromptSaving(false) }
  }

  const resetPrompt = () => {
    setAiPrompt(`你是一个专业的软件 Patch 分析助手。请根据以下 Patch 发布通知邮件内容，生成一份结构化的 Patch 调整摘要。

如果邮件正文中包含 WARP-xxxxx 格式的工单编号，请：
1. 使用 query_warp_issue 工具查询该工单在 JIRA 中的详细信息（标题、描述、状态、评论等）
2. 使用 search_wiki 工具搜索 Wiki 上与该 WARP 编号相关的文档和附件（如测试 SQL 文件、技术方案），搜索时直接传入 WARP 编号
3. 如果 search_wiki 返回了页面结果但内容不够详细，可使用 get_wiki_page 工具获取该页面的完整正文和附件内容。传入页面的 ID 即可

search_wiki 会自动获取页面正文和文本类附件（如 .sql、.txt、.properties）的内容，通常已包含足够信息。只有当内容被截断或需要更详细信息时，才需要使用 get_wiki_page。

结合 JIRA 工单和 Wiki 搜索结果，更全面地分析 Patch 调整的原因和影响。

重要：JIRA 工单内容和 Wiki 文档内容请直接内嵌展示在输出中，不要只放链接让用户跳转查看。用户应该在当前页面就能看到完整信息，原文链接仅作为参考附在末尾。

请按以下格式输出：

## Patch 基本信息
- 产品及版本
- Patch 类型（预览/通用/定向）
- Patch 日期

## JIRA 工单详情
对每个查询到的 WARP 工单，直接展示完整内容，格式如下：

### WARP-xxxxx：工单标题
- **状态**：工单状态
- **描述**：工单描述内容（直接贴出，不要省略）
- **关键评论**：贴出重要评论内容（如解决方案、根因分析等）
- **原文链接**：[WARP-xxxxx](JIRA 工单 URL)

## 调整内容
列出本次 Patch 涉及的主要调整和修复内容

## 影响范围
分析本次 Patch 可能影响的模块和功能

## 注意事项
部署或升级时需要注意的事项

## Wiki 相关信息
对每个搜索到的 Wiki 文档，直接展示内容，格式如下：

### 文档名称
直接贴出页面正文内容（不要省略或只写摘要）

如果是附件，按以下格式输出：

**附件：文件名**（如 .sql、.properties 等文本附件）
直接贴出附件原文内容，用代码块包裹，方便复制和执行

[查看 Wiki 原文](Wiki 页面 URL)

## 测试案例
根据 JIRA 工单描述、Wiki 文档内容（尤其是附件中的测试 SQL、配置文件等），为每个调整项生成对应的测试案例。格式如下：

### 测试案例 1：[案例名称]
- **关联 WARP 工单**：WARP-xxxxx
- **前置条件**：测试前需要准备的环境和数据
- **测试步骤**：
  1. 步骤一
  2. 步骤二
- **预期结果**：期望的测试结果
- **验证 SQL/脚本**：（如果 Wiki 附件中有测试 SQL，直接引用，用代码块包裹）

---

邮件内容：
`)
  }

  const startEdit = (cfg = null) => {
    if (cfg) {
      setEditing(cfg.id)
      setForm({ name: cfg.name, endpoint: cfg.endpoint, api_key: cfg.api_key, model: cfg.model, is_default: cfg.is_default })
    } else {
      setEditing('new')
      setForm({ name: '', endpoint: '', api_key: '', model: '', is_default: configs.length === 0 })
    }
  }

  const cancelEdit = () => { setEditing(null); setForm({ name: '', endpoint: '', api_key: '', model: '', is_default: false }) }

  const handleSave = async () => {
    if (!form.name || !form.endpoint || !form.api_key || !form.model) { alert('请填写所有必填项'); return }
    setSaving(true)
    try {
      if (editing === 'new') await aiApi.createConfig(form)
      else await aiApi.updateConfig(editing, form)
      cancelEdit()
      onRefresh()
    } catch (e) { alert('保存失败: ' + (e.message || '未知错误')) }
    finally { setSaving(false) }
  }

  const handleDelete = async (id) => {
    if (!confirm('确定删除此 AI 配置？')) return
    try { await aiApi.deleteConfig(id); onRefresh() }
    catch (e) { alert('删除失败: ' + (e.message || '未知错误')) }
  }

  const handleSetDefault = async (id) => {
    try { await aiApi.setDefault(id); onRefresh() }
    catch (e) { alert('设置失败: ' + (e.message || '未知错误')) }
  }

  const inputStyle = { width: '100%', padding: '8px 12px', border: '1px solid var(--border)', borderRadius: 'var(--radius)', fontSize: 13, outline: 'none', boxSizing: 'border-box' }

  return (
    <div style={{ position: 'fixed', top: 0, left: 0, right: 0, bottom: 0, zIndex: 2000, display: 'flex', alignItems: 'center', justifyContent: 'center' }}>
      <div onClick={onClose} style={{ position: 'absolute', top: 0, left: 0, right: 0, bottom: 0, backgroundColor: 'rgba(0,0,0,0.4)' }} />
      <div style={{ position: 'relative', background: '#fff', borderRadius: 16, width: 560, maxHeight: '80vh', boxShadow: '0 20px 60px rgba(0,0,0,0.2)', display: 'flex', flexDirection: 'column' }}>
        <div style={{ padding: '20px 24px', borderBottom: '1px solid var(--border)', display: 'flex', alignItems: 'center', justifyContent: 'space-between', background: 'linear-gradient(135deg, #F5F3FF 0%, #EEF2FF 100%)', borderRadius: '16px 16px 0 0' }}>
          <div style={{ display: 'flex', alignItems: 'center', gap: 10 }}>
            <Sparkles size={20} color="#7C3AED" />
            <h3 style={{ margin: 0, fontSize: 17, fontWeight: 600, color: '#5B21B6' }}>AI 服务配置</h3>
          </div>
          <button onClick={onClose} style={{ padding: 4, border: 'none', background: 'none', cursor: 'pointer', color: 'var(--text-muted)' }}><X size={18} /></button>
        </div>

        <div style={{ flex: 1, overflow: 'auto', padding: '20px 24px' }}>
          {configs.length === 0 ? (
            <div style={{ textAlign: 'center', padding: 24, color: 'var(--text-muted)' }}>
              <Sparkles size={32} style={{ marginBottom: 8 }} />
              <p>尚未配置 AI 服务</p>
              <p style={{ fontSize: 12 }}>支持 OpenAI 兼容接口（DeepSeek、通义千问等）</p>
            </div>
          ) : (
            <div style={{ display: 'flex', flexDirection: 'column', gap: 12, marginBottom: 16 }}>
              {configs.map(cfg => (
                <div key={cfg.id} style={{ padding: 16, border: '1px solid var(--border)', borderRadius: 'var(--radius-lg)', background: cfg.is_default ? '#F5F3FF' : '#FAFAFA', borderColor: cfg.is_default ? '#C4B5FD' : 'var(--border)' }}>
                  <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between' }}>
                    <div>
                      <div style={{ display: 'flex', alignItems: 'center', gap: 8 }}>
                        <span style={{ fontWeight: 600, fontSize: 14 }}>{cfg.name}</span>
                        {cfg.is_default && <span style={{ fontSize: 11, background: 'var(--gradient-primary)', color: '#fff', padding: '1px 6px', borderRadius: 4 }}>默认</span>}
                      </div>
                      <div style={{ fontSize: 12, color: 'var(--text-secondary)', marginTop: 4 }}>{cfg.endpoint} · {cfg.model}</div>
                    </div>
                    <div style={{ display: 'flex', gap: 6 }}>
                      {!cfg.is_default && <button onClick={() => handleSetDefault(cfg.id)} title="设为默认" style={{ padding: '4px 8px', border: '1px solid var(--border)', borderRadius: 'var(--radius)', background: '#fff', cursor: 'pointer', fontSize: 12, color: 'var(--text-secondary)', display: 'flex', alignItems: 'center', gap: 2 }}><Star size={12} /> 设为默认</button>}
                      <button onClick={() => startEdit(cfg)} style={{ padding: '4px 8px', border: '1px solid var(--border)', borderRadius: 'var(--radius)', background: '#fff', cursor: 'pointer', fontSize: 12, color: 'var(--primary)', display: 'flex', alignItems: 'center', gap: 2 }}>编辑</button>
                      <button onClick={() => handleDelete(cfg.id)} style={{ padding: '4px 8px', border: '1px solid #FECACA', borderRadius: 'var(--radius)', background: '#fff', cursor: 'pointer', fontSize: 12, color: '#DC2626', display: 'flex', alignItems: 'center', gap: 2 }}><Trash2 size={12} /> 删除</button>
                    </div>
                  </div>
                </div>
              ))}
            </div>
          )}

          {/* 提示词编辑区 */}
          <div style={{ marginTop: 16, padding: 16, border: '1px solid #E0E7FF', borderRadius: 'var(--radius-lg)', background: 'linear-gradient(135deg, #F8F9FF 0%, #F5F3FF 100%)' }}>
            <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', marginBottom: 10 }}>
              <div style={{ display: 'flex', alignItems: 'center', gap: 6 }}>
                <Edit3 size={14} color="#7C3AED" />
                <span style={{ fontSize: 13, fontWeight: 600, color: '#5B21B6' }}>AI 汇总提示词</span>
              </div>
              <div style={{ display: 'flex', gap: 6 }}>
                <button onClick={resetPrompt} style={{ padding: '3px 10px', border: '1px solid var(--border)', borderRadius: 'var(--radius)', background: '#fff', cursor: 'pointer', fontSize: 11, color: 'var(--text-secondary)' }}>恢复默认</button>
                <button onClick={savePrompt} disabled={promptSaving} style={{ padding: '3px 12px', border: 'none', borderRadius: 'var(--radius)', background: '#7C3AED', color: '#fff', cursor: promptSaving ? 'not-allowed' : 'pointer', fontSize: 11, fontWeight: 500, opacity: promptSaving ? 0.7 : 1 }}>
                  {promptSaving ? '保存中...' : (promptSaved ? '已保存 ✓' : '保存提示词')}
                </button>
              </div>
            </div>
            <textarea
              value={aiPrompt}
              onChange={e => setAiPrompt(e.target.value)}
              placeholder="输入 AI 汇总时使用的提示词..."
              style={{ width: '100%', minHeight: 160, padding: '10px 12px', border: '1px solid #C7D2FE', borderRadius: 'var(--radius)', fontSize: 12, lineHeight: 1.6, color: 'var(--text)', background: '#fff', outline: 'none', resize: 'vertical', boxSizing: 'border-box', fontFamily: 'inherit' }}
            />
            <div style={{ fontSize: 11, color: 'var(--text-muted)', marginTop: 6, lineHeight: 1.5 }}>
              提示词末尾会自动拼接邮件正文。留空则使用默认提示词。支持引导 AI 使用 query_warp_issue 工具查询 JIRA 工单。
            </div>
          </div>

          {editing ? (
            <div style={{ padding: 20, border: '1px solid #C4B5FD', borderRadius: 'var(--radius-lg)', background: '#FAFAFE' }}>
              <h4 style={{ margin: '0 0 16px', fontSize: 14, fontWeight: 600, color: '#5B21B6' }}>{editing === 'new' ? '添加 AI 配置' : '编辑配置'}</h4>
              <div style={{ display: 'flex', flexDirection: 'column', gap: 12 }}>
                <div>
                  <label style={{ fontSize: 12, fontWeight: 500, color: 'var(--text-secondary)', marginBottom: 4, display: 'block' }}>配置名称 *</label>
                  <input value={form.name} onChange={e => setForm({ ...form, name: e.target.value })} placeholder="如：DeepSeek" style={inputStyle} />
                </div>
                <div>
                  <label style={{ fontSize: 12, fontWeight: 500, color: 'var(--text-secondary)', marginBottom: 4, display: 'block' }}>API Endpoint *</label>
                  <input value={form.endpoint} onChange={e => setForm({ ...form, endpoint: e.target.value })} placeholder="https://api.deepseek.com/v1" style={inputStyle} />
                  <span style={{ fontSize: 11, color: 'var(--text-muted)', marginTop: 2, display: 'block' }}>Base URL 或完整路径均可，如 https://api.deepseek.com/v1 或 https://api.deepseek.com/v1/chat/completions</span>
                </div>
                <div>
                  <label style={{ fontSize: 12, fontWeight: 500, color: 'var(--text-secondary)', marginBottom: 4, display: 'block' }}>API Key *</label>
                  <input type="password" value={form.api_key} onChange={e => setForm({ ...form, api_key: e.target.value })} placeholder="sk-..." style={inputStyle} />
                </div>
                <div>
                  <label style={{ fontSize: 12, fontWeight: 500, color: 'var(--text-secondary)', marginBottom: 4, display: 'block' }}>模型名称 *</label>
                  <input value={form.model} onChange={e => setForm({ ...form, model: e.target.value })} placeholder="deepseek-chat / qwen-plus / gpt-4o" style={inputStyle} />
                </div>
                <div style={{ display: 'flex', alignItems: 'center', gap: 8 }}>
                  <input type="checkbox" checked={form.is_default} onChange={e => setForm({ ...form, is_default: e.target.checked })} id="is_default_cb" style={{ width: 16, height: 16, cursor: 'pointer' }} />
                  <label htmlFor="is_default_cb" style={{ fontSize: 13, cursor: 'pointer', color: 'var(--text)' }}>设为默认配置</label>
                </div>
                <div style={{ display: 'flex', gap: 8, justifyContent: 'flex-end', marginTop: 4 }}>
                  <button onClick={cancelEdit} style={{ padding: '8px 16px', border: '1px solid var(--border)', borderRadius: 'var(--radius)', background: '#fff', cursor: 'pointer', fontSize: 13, color: 'var(--text-secondary)' }}>取消</button>
                  <button onClick={handleSave} disabled={saving} style={{ padding: '8px 20px', border: 'none', borderRadius: 'var(--radius)', background: '#7C3AED', color: '#fff', cursor: saving ? 'not-allowed' : 'pointer', fontSize: 13, fontWeight: 500, opacity: saving ? 0.7 : 1 }}>{saving ? '保存中...' : '保存'}</button>
                </div>
              </div>
            </div>
          ) : (
            <button onClick={() => startEdit()} style={{ width: '100%', padding: '10px 16px', border: '1px dashed #C4B5FD', borderRadius: 'var(--radius-lg)', background: '#FAFAFE', cursor: 'pointer', fontSize: 13, color: '#7C3AED', fontWeight: 500, display: 'flex', alignItems: 'center', justifyContent: 'center', gap: 6 }}><Plus size={14} /> 添加 AI 配置</button>
          )}
        </div>
      </div>
    </div>
  )
}
