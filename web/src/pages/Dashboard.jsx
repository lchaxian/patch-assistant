import { useState, useEffect } from 'react'
import { statsApi, mailApi } from '../api'
import { Users, Mail, MailOpen, Calendar, TrendingUp } from 'lucide-react'
import dayjs from 'dayjs'

export default function Dashboard() {
  const [stats, setStats] = useState(null)
  const [summaries, setSummaries] = useState([])
  const [loading, setLoading] = useState(true)

  useEffect(() => {
    loadData()
  }, [])

  async function loadData() {
    try {
      const [statsRes, summaryRes] = await Promise.all([
        statsApi.overview(),
        mailApi.summary(),
      ])
      setStats(statsRes.data)
      setSummaries(summaryRes.data || [])
    } catch (err) {
      console.error('加载统计数据失败:', err)
    } finally {
      setLoading(false)
    }
  }

  if (loading) {
    return <div style={{ textAlign: 'center', padding: 48 }}><div className="loading-spinner" /></div>
  }

  return (
    <div>
      <div className="page-header">
        <div>
          <h2>总览</h2>
          <p>企业邮箱信息汇总面板</p>
        </div>
      </div>

      <div className="stats-grid">
        <div className="stat-card">
          <div className="stat-icon" style={{ background: '#EEF2FF' }}>
            <Users size={20} style={{ color: '#4F46E5' }} />
          </div>
          <div className="stat-value">{stats?.total_accounts || 0}</div>
          <div className="stat-label">邮箱账户</div>
        </div>
        <div className="stat-card">
          <div className="stat-icon" style={{ background: '#ECFDF5' }}>
            <Mail size={20} style={{ color: '#10B981' }} />
          </div>
          <div className="stat-value">{stats?.total_mails || 0}</div>
          <div className="stat-label">总邮件数</div>
        </div>
        <div className="stat-card">
          <div className="stat-icon" style={{ background: '#FEF3C7' }}>
            <MailOpen size={20} style={{ color: '#F59E0B' }} />
          </div>
          <div className="stat-value">{stats?.unread_mails || 0}</div>
          <div className="stat-label">未读邮件</div>
        </div>
        <div className="stat-card">
          <div className="stat-icon" style={{ background: '#EFF6FF' }}>
            <Calendar size={20} style={{ color: '#3B82F6' }} />
          </div>
          <div className="stat-value">{stats?.today_mails || 0}</div>
          <div className="stat-label">今日新邮件</div>
        </div>
        <div className="stat-card">
          <div className="stat-icon" style={{ background: '#FDF2F8' }}>
            <TrendingUp size={20} style={{ color: '#EC4899' }} />
          </div>
          <div className="stat-value">{stats?.week_mails || 0}</div>
          <div className="stat-label">本周邮件</div>
        </div>
      </div>

      <div className="card">
        <div className="card-header">
          <h3>各账户邮件统计</h3>
        </div>
        {summaries.length === 0 ? (
          <div className="empty-state">
            <Mail size={48} />
            <p>暂无数据，请先添加邮箱账户并同步邮件</p>
          </div>
        ) : (
          <div className="table-wrapper">
            <table>
              <thead>
                <tr>
                  <th>邮箱账户</th>
                  <th>显示名称</th>
                  <th>总邮件</th>
                  <th>未读</th>
                  <th>今日</th>
                  <th>最后同步</th>
                </tr>
              </thead>
              <tbody>
                {summaries.map((s) => (
                  <tr key={s.account_id}>
                    <td style={{ fontWeight: 500 }}>{s.email}</td>
                    <td>{s.display_name || '-'}</td>
                    <td>{s.total_mails}</td>
                    <td>
                      {s.unread_mails > 0 ? (
                        <span className="badge badge-warning">{s.unread_mails}</span>
                      ) : '0'}
                    </td>
                    <td>{s.today_mails}</td>
                    <td>{s.last_sync_at ? dayjs(s.last_sync_at).format('YYYY-MM-DD HH:mm') : '未同步'}</td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        )}
      </div>
    </div>
  )
}
