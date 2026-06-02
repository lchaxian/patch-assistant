import { useState, useEffect } from 'react'
import { accountApi, mailApi } from '../api'
import { Plus, Trash2, Edit3, Plug, RefreshCw, CheckCircle, XCircle } from 'lucide-react'
import dayjs from 'dayjs'

const DEFAULT_IMAP = { host: 'imap.exmail.qq.com', port: 993 }

export default function Accounts() {
  const [accounts, setAccounts] = useState([])
  const [loading, setLoading] = useState(true)
  const [showModal, setShowModal] = useState(false)
  const [editing, setEditing] = useState(null)
  const [form, setForm] = useState({ email: '', display_name: '', password: '', imap_host: DEFAULT_IMAP.host, imap_port: DEFAULT_IMAP.port, use_tls: true })
  const [testing, setTesting] = useState(null)
  const [syncing, setSyncing] = useState(null)
  const [testResult, setTestResult] = useState({})

  useEffect(() => {
    loadAccounts()
  }, [])

  async function loadAccounts() {
    try {
      const res = await accountApi.list()
      setAccounts(res.data || [])
    } catch (err) {
      console.error('加载账户失败:', err)
    } finally {
      setLoading(false)
    }
  }

  function openCreate() {
    setEditing(null)
    setForm({ email: '', display_name: '', password: '', imap_host: DEFAULT_IMAP.host, imap_port: DEFAULT_IMAP.port, use_tls: true })
    setShowModal(true)
  }

  function openEdit(acc) {
    setEditing(acc)
    setForm({
      email: acc.email,
      display_name: acc.display_name,
      password: '',
      imap_host: acc.imap_host,
      imap_port: acc.imap_port,
      use_tls: acc.use_tls,
    })
    setShowModal(true)
  }

  async function handleSubmit(e) {
    e.preventDefault()
    try {
      if (editing) {
        await accountApi.update(editing.id, form)
      } else {
        await accountApi.create(form)
      }
      setShowModal(false)
      loadAccounts()
    } catch (err) {
      alert('操作失败: ' + err.message)
    }
  }

  async function handleDelete(id) {
    if (!confirm('确定要删除此账户吗？关联的邮件数据也将被删除。')) return
    try {
      await accountApi.delete(id)
      loadAccounts()
    } catch (err) {
      alert('删除失败: ' + err.message)
    }
  }

  async function handleTest(id) {
    setTesting(id)
    setTestResult({})
    try {
      const res = await accountApi.test(id)
      setTestResult({ [id]: { success: res.success, message: res.message || res.error } })
    } catch (err) {
      setTestResult({ [id]: { success: false, message: err.message } })
    } finally {
      setTesting(null)
    }
  }

  async function handleSync(id) {
    setSyncing(id)
    try {
      const res = await mailApi.sync(id)
      alert(`同步完成！新增 ${res.data?.new_mails || 0} 封邮件，共 ${res.data?.total_mails || 0} 封`)
      loadAccounts()
    } catch (err) {
      alert('同步失败: ' + err.message)
    } finally {
      setSyncing(null)
    }
  }

  if (loading) {
    return <div style={{ textAlign: 'center', padding: 48 }}><div className="loading-spinner" /></div>
  }

  return (
    <div>
      <div className="page-header">
        <div>
          <h2>邮箱账户</h2>
          <p>管理腾讯企业邮箱账户，支持 IMAP 协议连接</p>
        </div>
        <button className="btn btn-primary" onClick={openCreate}>
          <Plus size={16} /> 添加账户
        </button>
      </div>

      {accounts.length === 0 ? (
        <div className="card">
          <div className="empty-state">
            <Plug size={48} />
            <p>还没有添加邮箱账户</p>
            <button className="btn btn-primary" onClick={openCreate}>
              <Plus size={16} /> 添加第一个账户
            </button>
          </div>
        </div>
      ) : (
        <div className="card">
          <div className="table-wrapper">
            <table>
              <thead>
                <tr>
                  <th>邮箱地址</th>
                  <th>显示名称</th>
                  <th>IMAP 服务器</th>
                  <th>状态</th>
                  <th>最后同步</th>
                  <th>操作</th>
                </tr>
              </thead>
              <tbody>
                {accounts.map((acc) => (
                  <tr key={acc.id}>
                    <td style={{ fontWeight: 500 }}>{acc.email}</td>
                    <td>{acc.display_name || '-'}</td>
                    <td>{acc.imap_host}:{acc.imap_port}</td>
                    <td>
                      <span className={`badge ${acc.status === 'active' ? 'badge-success' : acc.status === 'error' ? 'badge-danger' : 'badge-warning'}`}>
                        {acc.status === 'active' ? '正常' : acc.status === 'error' ? '异常' : '未验证'}
                      </span>
                      {testResult[acc.id] && (
                        <span style={{ marginLeft: 8 }}>
                          {testResult[acc.id].success
                            ? <CheckCircle size={14} style={{ color: '#10B981' }} />
                            : <XCircle size={14} style={{ color: '#EF4444' }} />}
                        </span>
                      )}
                    </td>
                    <td>{acc.last_sync_at ? dayjs(acc.last_sync_at).format('YYYY-MM-DD HH:mm') : '未同步'}</td>
                    <td>
                      <div style={{ display: 'flex', gap: 4 }}>
                        <button className="btn btn-secondary btn-sm" onClick={() => handleTest(acc.id)} disabled={testing === acc.id}>
                          {testing === acc.id ? <div className="loading-spinner" /> : <Plug size={14} />} 测试
                        </button>
                        <button className="btn btn-secondary btn-sm" onClick={() => handleSync(acc.id)} disabled={syncing === acc.id}>
                          {syncing === acc.id ? <div className="loading-spinner" /> : <RefreshCw size={14} />} 同步
                        </button>
                        <button className="btn btn-secondary btn-sm" onClick={() => openEdit(acc)}>
                          <Edit3 size={14} />
                        </button>
                        <button className="btn btn-danger btn-sm" onClick={() => handleDelete(acc.id)}>
                          <Trash2 size={14} />
                        </button>
                      </div>
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        </div>
      )}

      {showModal && (
        <div className="modal-overlay" onClick={(e) => e.target === e.currentTarget && setShowModal(false)}>
          <div className="modal">
            <h3>{editing ? '编辑账户' : '添加邮箱账户'}</h3>
            <form onSubmit={handleSubmit}>
              <div className="form-group">
                <label>邮箱地址</label>
                <input
                  type="email"
                  placeholder="user@company.com"
                  value={form.email}
                  onChange={(e) => setForm({ ...form, email: e.target.value })}
                  required
                />
              </div>
              <div className="form-group">
                <label>显示名称</label>
                <input
                  type="text"
                  placeholder="可选，方便识别"
                  value={form.display_name}
                  onChange={(e) => setForm({ ...form, display_name: e.target.value })}
                />
              </div>
              <div className="form-group">
                <label>{editing ? '新密码（留空不修改）' : '邮箱密码'}</label>
                <input
                  type="password"
                  placeholder={editing ? '留空则不修改密码' : '输入邮箱密码'}
                  value={form.password}
                  onChange={(e) => setForm({ ...form, password: e.target.value })}
                  required={!editing}
                />
              </div>
              <div className="form-row">
                <div className="form-group">
                  <label>IMAP 服务器</label>
                  <input
                    type="text"
                    value={form.imap_host}
                    onChange={(e) => setForm({ ...form, imap_host: e.target.value })}
                  />
                </div>
                <div className="form-group">
                  <label>端口</label>
                  <input
                    type="number"
                    value={form.imap_port}
                    onChange={(e) => setForm({ ...form, imap_port: parseInt(e.target.value) || 993 })}
                  />
                </div>
              </div>
              <div className="form-group">
                <label style={{ display: 'flex', alignItems: 'center', gap: 8, fontWeight: 400 }}>
                  <input
                    type="checkbox"
                    checked={form.use_tls}
                    onChange={(e) => setForm({ ...form, use_tls: e.target.checked })}
                  />
                  使用 SSL/TLS 加密连接
                </label>
              </div>
              <div className="modal-actions">
                <button type="button" className="btn btn-secondary" onClick={() => setShowModal(false)}>取消</button>
                <button type="submit" className="btn btn-primary">{editing ? '保存' : '添加'}</button>
              </div>
            </form>
          </div>
        </div>
      )}
    </div>
  )
}
