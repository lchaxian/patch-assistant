import { useState, useEffect } from 'react'
import { accountApi, mailApi } from '../api'
import { Search, RefreshCw, ChevronLeft, ChevronRight, Mail, Paperclip, X, User, Clock, ArrowLeft } from 'lucide-react'
import dayjs from 'dayjs'

export default function Mails() {
  const [accounts, setAccounts] = useState([])
  const [selectedAccount, setSelectedAccount] = useState(null)
  const [mails, setMails] = useState([])
  const [pagination, setPagination] = useState({ page: 1, page_size: 50, total: 0, total_page: 0 })
  const [keyword, setSearchKeyword] = useState('')
  const [loading, setLoading] = useState(false)
  const [syncing, setSyncing] = useState(false)
  const [selectedMail, setSelectedMail] = useState(null)
  const [mailDetail, setMailDetail] = useState(null)
  const [loadingDetail, setLoadingDetail] = useState(false)

  useEffect(() => {
    loadAccounts()
  }, [])

  async function loadAccounts() {
    try {
      const res = await accountApi.list()
      const list = res.data || []
      setAccounts(list)
      if (list.length > 0 && !selectedAccount) {
        setSelectedAccount(list[0].id)
      }
    } catch (err) {
      console.error('加载账户失败:', err)
    }
  }

  useEffect(() => {
    if (selectedAccount) {
      loadMails(1)
    }
  }, [selectedAccount])

  async function loadMails(page) {
    if (!selectedAccount) return
    setLoading(true)
    try {
      const params = { page, page_size: pagination.page_size }
      if (keyword) params.keyword = keyword
      const res = await mailApi.list(selectedAccount, params)
      setMails(res.data || [])
      setPagination(res.pagination || pagination)
    } catch (err) {
      console.error('加载邮件失败:', err)
    } finally {
      setLoading(false)
    }
  }

  async function handleSync() {
    if (!selectedAccount) return
    setSyncing(true)
    try {
      const res = await mailApi.sync(selectedAccount, 30)
      alert(`同步完成！新增 ${res.data?.new_mails || 0} 封邮件`)
      loadMails(1)
    } catch (err) {
      alert('同步失败: ' + err.message)
    } finally {
      setSyncing(false)
    }
  }

  async function handleMailClick(mail) {
    setSelectedMail(mail)
    setLoadingDetail(true)
    setMailDetail(null)
    try {
      const res = await mailApi.detail(mail.id)
      setMailDetail(res.data || null)
    } catch (err) {
      console.error('获取邮件详情失败:', err)
      setMailDetail(null)
    } finally {
      setLoadingDetail(false)
    }
  }

  function handleCloseDetail() {
    setSelectedMail(null)
    setMailDetail(null)
  }

  function handleSearch(e) {
    e.preventDefault()
    loadMails(1)
  }

  function getInitial(name) {
    if (!name) return '?'
    return name.charAt(0).toUpperCase()
  }

  function formatSize(bytes) {
    if (!bytes) return ''
    if (bytes < 1024) return bytes + ' B'
    if (bytes < 1024 * 1024) return (bytes / 1024).toFixed(1) + ' KB'
    return (bytes / 1024 / 1024).toFixed(1) + ' MB'
  }

  // 邮件详情面板
  function MailDetailPanel() {
    if (!selectedMail) return null

    return (
      <div style={{
        position: 'fixed', top: 0, right: 0, bottom: 0, width: '60%', minWidth: 480,
        backgroundColor: '#FFFFFF', borderLeft: '1px solid var(--border)',
        boxShadow: '-4px 0 24px rgba(0,0,0,0.12)', zIndex: 1000, display: 'flex',
        flexDirection: 'column', overflow: 'hidden'
      }}>
        {/* 头部 */}
        <div style={{
          padding: '16px 24px', borderBottom: '1px solid var(--border)',
          display: 'flex', alignItems: 'center', gap: 12,
          backgroundColor: '#F8FAFC'
        }}>
          <button onClick={handleCloseDetail} style={{
            display: 'flex', alignItems: 'center', gap: 4, padding: '6px 12px',
            border: '1px solid var(--border)', borderRadius: 'var(--radius)',
            background: '#FFFFFF', cursor: 'pointer', fontSize: 13,
            color: 'var(--text-secondary)'
          }}>
            <ArrowLeft size={14} /> 返回
          </button>
          <span style={{ flex: 1 }}></span>
          <button onClick={handleCloseDetail} style={{
            padding: 4, border: 'none', background: 'none', cursor: 'pointer',
            color: 'var(--text-muted)'
          }}>
            <X size={18} />
          </button>
        </div>

        {/* 邮件信息 */}
        <div style={{ padding: '20px 24px', borderBottom: '1px solid var(--border)' }}>
          <h2 style={{ margin: '0 0 12px 0', fontSize: 18, fontWeight: 600, color: 'var(--text)' }}>
            {selectedMail.subject || '(无主题)'}
          </h2>
          <div style={{ display: 'flex', alignItems: 'center', gap: 12, marginBottom: 8 }}>
            <div style={{
              width: 36, height: 36, borderRadius: '50%',
              backgroundColor: 'var(--primary)', color: '#fff',
              display: 'flex', alignItems: 'center', justifyContent: 'center',
              fontSize: 14, fontWeight: 600, flexShrink: 0
            }}>
              {getInitial(selectedMail.from_name || selectedMail.from_addr)}
            </div>
            <div>
              <div style={{ fontWeight: 500, fontSize: 14 }}>{selectedMail.from_name || selectedMail.from_addr}</div>
              <div style={{ fontSize: 12, color: 'var(--text-muted)' }}>&lt;{selectedMail.from_addr}&gt;</div>
            </div>
            <span style={{ marginLeft: 'auto', fontSize: 12, color: 'var(--text-muted)' }}>
              {dayjs(selectedMail.date).format('YYYY-MM-DD HH:mm')}
            </span>
          </div>
          {selectedMail.to_addr && (
            <div style={{ fontSize: 12, color: 'var(--text-muted)' }}>
              收件人: {selectedMail.to_addr}
            </div>
          )}
        </div>

        {/* 正文 */}
        <div style={{ flex: 1, overflow: 'auto', padding: '20px 24px' }}>
          {loadingDetail ? (
            <div style={{ textAlign: 'center', padding: 48 }}>
              <div className="loading-spinner" />
              <p style={{ color: 'var(--text-muted)', marginTop: 12 }}>加载邮件内容...</p>
            </div>
          ) : mailDetail ? (
            <MailBodyContent detail={mailDetail} />
          ) : (
            <div style={{ textAlign: 'center', padding: 48, color: 'var(--text-muted)' }}>
              <Mail size={36} />
              <p style={{ marginTop: 12 }}>无法加载邮件内容</p>
              <p style={{ fontSize: 12 }}>请重新同步邮件后再试</p>
            </div>
          )}
        </div>
      </div>
    )
  }

  // 正文内容渲染
  function MailBodyContent({ detail }) {
    const [showHtml, setShowHtml] = useState(true)

    // 优先显示 HTML，如果没有则显示纯文本
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
        {/* 切换按钮 */}
        {hasHtml && hasText && (
          <div style={{ marginBottom: 12, display: 'flex', gap: 8 }}>
            <button
              onClick={() => setShowHtml(true)}
              style={{
                padding: '4px 12px', fontSize: 12, borderRadius: 'var(--radius)',
                border: '1px solid var(--border)', cursor: 'pointer',
                background: showHtml ? 'var(--primary)' : '#FFFFFF',
                color: showHtml ? '#fff' : 'var(--text-secondary)'
              }}
            >
              富文本
            </button>
            <button
              onClick={() => setShowHtml(false)}
              style={{
                padding: '4px 12px', fontSize: 12, borderRadius: 'var(--radius)',
                border: '1px solid var(--border)', cursor: 'pointer',
                background: !showHtml ? 'var(--primary)' : '#FFFFFF',
                color: !showHtml ? '#fff' : 'var(--text-secondary)'
              }}
            >
              纯文本
            </button>
          </div>
        )}

        {/* 正文内容 */}
        {showHtml && hasHtml ? (
          <div style={{
            border: '1px solid var(--border)', borderRadius: 'var(--radius)',
            overflow: 'hidden', backgroundColor: '#fff'
          }}>
            <iframe
              srcDoc={detail.body_html}
              title="邮件正文"
              style={{
                width: '100%', minHeight: 400, border: 'none',
                display: 'block'
              }}
              sandbox="allow-same-origin"
              onLoad={(e) => {
                // 自动调整 iframe 高度
                const iframe = e.target
                try {
                  const body = iframe.contentDocument?.body
                  if (body) {
                    iframe.style.height = Math.max(body.scrollHeight + 20, 400) + 'px'
                  }
                } catch (err) {
                  // 跨域限制，忽略
                }
              }}
            />
          </div>
        ) : (
          <pre style={{
            whiteSpace: 'pre-wrap', wordBreak: 'break-word', fontSize: 14,
            lineHeight: 1.7, color: 'var(--text)',
            backgroundColor: '#F8FAFC', padding: 16,
            borderRadius: 'var(--radius)', border: '1px solid var(--border)',
            margin: 0, fontFamily: 'inherit'
          }}>
            {detail.body_text}
          </pre>
        )}
      </div>
    )
  }

  // 遮罩层
  const DetailOverlay = selectedMail ? (
    <div
      onClick={handleCloseDetail}
      style={{
        position: 'fixed', top: 0, left: 0, right: 0, bottom: 0,
        backgroundColor: 'rgba(0,0,0,0.3)', zIndex: 999
      }}
    />
  ) : null

  return (
    <div>
      <div className="page-header">
        <div>
          <h2>邮件列表</h2>
          <p>查看和搜索已同步的邮件</p>
        </div>
        <button className="btn btn-primary" onClick={handleSync} disabled={syncing || !selectedAccount}>
          {syncing ? <div className="loading-spinner" /> : <RefreshCw size={16} />}
          同步邮件
        </button>
      </div>

      {/* 筛选栏 */}
      <div className="card" style={{ marginBottom: 16, padding: '12px 16px' }}>
        <div style={{ display: 'flex', gap: 12, alignItems: 'center', flexWrap: 'wrap' }}>
          <select
            value={selectedAccount || ''}
            onChange={(e) => setSelectedAccount(Number(e.target.value))}
            style={{ padding: '8px 12px', border: '1px solid var(--border)', borderRadius: 'var(--radius)', fontSize: 14, minWidth: 240 }}
          >
            <option value="">选择邮箱账户</option>
            {accounts.map((acc) => (
              <option key={acc.id} value={acc.id}>{acc.email}</option>
            ))}
          </select>

          <form onSubmit={handleSearch} style={{ display: 'flex', gap: 8, flex: 1, maxWidth: 400 }}>
            <div className="search-box" style={{ flex: 1 }}>
              <Search size={16} />
              <input
                type="text"
                placeholder="搜索邮件主题或发件人..."
                value={keyword}
                onChange={(e) => setSearchKeyword(e.target.value)}
                style={{ width: '100%', padding: '8px 12px 8px 36px', border: '1px solid var(--border)', borderRadius: 'var(--radius)', fontSize: 14 }}
              />
            </div>
            <button type="submit" className="btn btn-secondary btn-sm">搜索</button>
          </form>

          {pagination.total > 0 && (
            <span style={{ fontSize: 13, color: 'var(--text-muted)', marginLeft: 'auto' }}>
              共 {pagination.total} 封邮件
            </span>
          )}
        </div>
      </div>

      {/* 邮件列表 */}
      <div className="card" style={{ padding: 0, overflow: 'hidden' }}>
        {loading ? (
          <div style={{ textAlign: 'center', padding: 48 }}><div className="loading-spinner" /></div>
        ) : !selectedAccount ? (
          <div className="empty-state">
            <Mail size={48} />
            <p>请先选择一个邮箱账户</p>
          </div>
        ) : mails.length === 0 ? (
          <div className="empty-state">
            <Mail size={48} />
            <p>暂无邮件数据，请先同步邮件</p>
            <button className="btn btn-primary" onClick={handleSync} disabled={syncing}>
              <RefreshCw size={16} /> 同步邮件
            </button>
          </div>
        ) : (
          <>
            {mails.map((mail) => (
              <div
                key={mail.id}
                className={`mail-item ${!mail.is_read ? 'unread' : ''}`}
                onClick={() => handleMailClick(mail)}
                style={{ cursor: 'pointer' }}
              >
                <div className="mail-avatar">
                  {getInitial(mail.from_name || mail.from_addr)}
                </div>
                <div className="mail-content">
                  <div className="mail-subject">{mail.subject || '(无主题)'}</div>
                  <div className="mail-meta">
                    <span>{mail.from_name || mail.from_addr}</span>
                    {mail.has_attach && <Paperclip size={12} style={{ color: 'var(--text-muted)' }} />}
                    <span>{formatSize(mail.size)}</span>
                  </div>
                </div>
                <div className="mail-date">
                  {dayjs(mail.date).format('MM-DD HH:mm')}
                </div>
              </div>
            ))}

            {/* 分页 */}
            {pagination.total_page > 1 && (
              <div style={{ display: 'flex', justifyContent: 'center', alignItems: 'center', gap: 12, padding: '16px' }}>
                <button
                  className="btn btn-secondary btn-sm"
                  disabled={pagination.page <= 1}
                  onClick={() => loadMails(pagination.page - 1)}
                >
                  <ChevronLeft size={14} /> 上一页
                </button>
                <span style={{ fontSize: 13, color: 'var(--text-secondary)' }}>
                  第 {pagination.page} / {pagination.total_page} 页
                </span>
                <button
                  className="btn btn-secondary btn-sm"
                  disabled={pagination.page >= pagination.total_page}
                  onClick={() => loadMails(pagination.page + 1)}
                >
                  下一页 <ChevronRight size={14} />
                </button>
              </div>
            )}
          </>
        )}
      </div>

      {/* 邮件详情面板 */}
      {DetailOverlay}
      <MailDetailPanel />
    </div>
  )
}
