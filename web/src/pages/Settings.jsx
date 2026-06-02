import { useState, useEffect } from 'react'
import { accountApi, jiraApi, aiApi } from '../api'
import {
  Users, Eye, EyeOff, CheckCircle, AlertCircle, X, Plus, Trash2, Edit3,
  Star, Sparkles, Settings as SettingsIcon, Mail, Server
} from 'lucide-react'
import dayjs from 'dayjs'

const TABS = [
  { key: 'accounts', label: '邮箱账户', icon: Users },
  { key: 'jira', label: 'Jira 配置', icon: Server },
  { key: 'ai', label: 'AI 配置', icon: Sparkles },
]

const DEFAULT_IMAP = { host: 'imap.exmail.qq.com', port: 993 }

export default function Settings() {
  const [activeTab, setActiveTab] = useState('accounts')

  return (
    <div>
      <div className="page-header">
        <div>
          <h2>设置</h2>
          <p>配置邮箱账户、Jira 凭据和 AI 服务，为 Patch 分析提供数据支撑</p>
        </div>
      </div>

      {/* Tabs */}
      <div style={{ display: 'flex', gap: 2, marginBottom: 24, background: '#F1F5F9', borderRadius: 'var(--radius-lg)', padding: 4 }}>
        {TABS.map(tab => {
          const Icon = tab.icon
          const isActive = activeTab === tab.key
          return (
            <button
              key={tab.key}
              onClick={() => setActiveTab(tab.key)}
              style={{
                display: 'inline-flex', alignItems: 'center', gap: 6,
                padding: '9px 18px', fontSize: 14, fontWeight: 500,
                cursor: 'pointer', border: 'none',
                borderRadius: isActive ? 'var(--radius)' : 'var(--radius)',
                background: isActive ? '#fff' : 'transparent',
                color: isActive ? 'var(--primary)' : 'var(--text-secondary)',
                transition: 'all 0.15s',
                boxShadow: isActive ? 'var(--shadow-sm)' : 'none',
                flex: 1,
                justifyContent: 'center',
              }}
            >
              <Icon size={16} /> {tab.label}
            </button>
          )
        })}
      </div>

      {activeTab === 'accounts' && <AccountsSection />}
      {activeTab === 'jira' && <JiraSection />}
      {activeTab === 'ai' && <AISection />}
    </div>
  )
}

/* ========== 邮箱账户 ========== */

function AccountsSection() {
  const [accounts, setAccounts] = useState([])
  const [loading, setLoading] = useState(true)
  const [showModal, setShowModal] = useState(false)
  const [editing, setEditing] = useState(null)
  const [form, setForm] = useState({ email: '', display_name: '', password: '', imap_host: DEFAULT_IMAP.host, imap_port: DEFAULT_IMAP.port, use_tls: true })
  const [testing, setTesting] = useState(null)
  const [syncing, setSyncing] = useState(null)
  const [testResult, setTestResult] = useState({})

  useEffect(() => { loadAccounts() }, [])

  const loadAccounts = async () => {
    try {
      const res = await accountApi.list()
      setAccounts(res.data || [])
    } catch (err) {
      console.error('加载账户失败:', err)
    } finally {
      setLoading(false)
    }
  }

  const openCreate = () => {
    setEditing(null)
    setForm({ email: '', display_name: '', password: '', imap_host: DEFAULT_IMAP.host, imap_port: DEFAULT_IMAP.port, use_tls: true })
    setShowModal(true)
  }

  const openEdit = (acc) => {
    setEditing(acc)
    setForm({ email: acc.email, display_name: acc.display_name, password: '', imap_host: acc.imap_host, imap_port: acc.imap_port, use_tls: acc.use_tls })
    setShowModal(true)
  }

  const handleSubmit = async (e) => {
    e.preventDefault()
    try {
      if (editing) await accountApi.update(editing.id, form)
      else await accountApi.create(form)
      setShowModal(false)
      loadAccounts()
    } catch (err) {
      alert('操作失败: ' + err.message)
    }
  }

  const handleDelete = async (id) => {
    if (!confirm('确定要删除此账户吗？关联的邮件数据也将被删除。')) return
    try { await accountApi.delete(id); loadAccounts() }
    catch (err) { alert('删除失败: ' + err.message) }
  }

  const handleTest = async (id) => {
    setTesting(id); setTestResult({})
    try {
      const res = await accountApi.test(id)
      setTestResult({ [id]: { success: res.success, message: res.message || res.error } })
    } catch (err) {
      setTestResult({ [id]: { success: false, message: err.message } })
    } finally { setTesting(null) }
  }

  const handleSync = async (id) => {
    setSyncing(id)
    try {
      const res = await accountApi.test(id) // placeholder — sync via mails
      const syncRes = await (await import('../api')).mailApi.sync(id, 30)
      alert(`同步完成！新增 ${syncRes.data?.new_mails || 0} 封邮件`)
      loadAccounts()
    } catch (err) {
      alert('同步失败: ' + err.message)
    } finally { setSyncing(null) }
  }

  if (loading) return <div style={{ textAlign: 'center', padding: 48 }}><div className="loading-spinner" /></div>

  return (
    <div>
      <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: 16 }}>
        <p style={{ fontSize: 14, color: 'var(--text-secondary)', margin: 0 }}>配置邮箱账户用于同步 Patch 发布通知邮件</p>
        <button className="btn btn-primary" onClick={openCreate}><Plus size={16} /> 添加账户</button>
      </div>

      {accounts.length === 0 ? (
        <div className="card"><div className="empty-state"><Users size={48} /><p>还没有添加邮箱账户</p><button className="btn btn-primary" onClick={openCreate}><Plus size={16} /> 添加第一个账户</button></div></div>
      ) : (
        <div className="card" style={{ padding: 0, overflow: 'hidden' }}>
          <table>
            <thead><tr><th>邮箱地址</th><th>显示名称</th><th>IMAP 服务器</th><th>状态</th><th>最后同步</th><th>操作</th></tr></thead>
            <tbody>
              {accounts.map(acc => (
                <tr key={acc.id}>
                  <td style={{ fontWeight: 500 }}>{acc.email}</td>
                  <td>{acc.display_name || '-'}</td>
                  <td>{acc.imap_host}:{acc.imap_port}</td>
                  <td>
                    <span className={`badge ${acc.status === 'active' ? 'badge-success' : 'badge-danger'}`}>
                      {acc.status === 'active' ? '正常' : '异常'}
                    </span>
                    {testResult[acc.id] && (
                      <span style={{ marginLeft: 8 }}>
                        {testResult[acc.id].success ? <CheckCircle size={14} style={{ color: '#10B981' }} /> : <AlertCircle size={14} style={{ color: '#EF4444' }} />}
                      </span>
                    )}
                  </td>
                  <td>{acc.last_sync_at ? dayjs(acc.last_sync_at).format('YYYY-MM-DD HH:mm') : '未同步'}</td>
                  <td>
                    <div style={{ display: 'flex', gap: 4 }}>
                      <button className="btn btn-secondary btn-sm" onClick={() => handleTest(acc.id)} disabled={testing === acc.id}>
                        {testing === acc.id ? <div className="loading-spinner" /> : <Mail size={14} />} 测试
                      </button>
                      <button className="btn btn-secondary btn-sm" onClick={() => handleSync(acc.id)} disabled={syncing === acc.id}>
                        {syncing === acc.id ? <div className="loading-spinner" /> : <Edit3 size={14} />} 同步
                      </button>
                      <button className="btn btn-secondary btn-sm" onClick={() => openEdit(acc)}><Edit3 size={14} /></button>
                      <button className="btn btn-danger btn-sm" onClick={() => handleDelete(acc.id)}><Trash2 size={14} /></button>
                    </div>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}

      {showModal && (
        <div className="modal-overlay" onClick={(e) => e.target === e.currentTarget && setShowModal(false)}>
          <div className="modal">
            <h3>{editing ? '编辑账户' : '添加邮箱账户'}</h3>
            <form onSubmit={handleSubmit}>
              <div className="form-group"><label>邮箱地址</label><input type="email" placeholder="user@company.com" value={form.email} onChange={(e) => setForm({ ...form, email: e.target.value })} required /></div>
              <div className="form-group"><label>显示名称</label><input type="text" placeholder="可选" value={form.display_name} onChange={(e) => setForm({ ...form, display_name: e.target.value })} /></div>
              <div className="form-group"><label>{editing ? '新密码（留空不修改）' : '邮箱密码'}</label><input type="password" placeholder={editing ? '留空则不修改' : '输入邮箱密码'} value={form.password} onChange={(e) => setForm({ ...form, password: e.target.value })} required={!editing} /></div>
              <div className="form-row">
                <div className="form-group"><label>IMAP 服务器</label><input type="text" value={form.imap_host} onChange={(e) => setForm({ ...form, imap_host: e.target.value })} /></div>
                <div className="form-group"><label>端口</label><input type="number" value={form.imap_port} onChange={(e) => setForm({ ...form, imap_port: parseInt(e.target.value) || 993 })} /></div>
              </div>
              <div className="form-group"><label style={{ display: 'flex', alignItems: 'center', gap: 8, fontWeight: 400 }}><input type="checkbox" checked={form.use_tls} onChange={(e) => setForm({ ...form, use_tls: e.target.checked })} /> 使用 SSL/TLS</label></div>
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

/* ========== Jira 配置 ========== */

function JiraSection() {
  const [config, setConfig] = useState(null)
  const [form, setForm] = useState({ username: '', password: '', base_url: 'https://jira.transwarp.io', login_url: 'https://erp.transwarp.io/api/v1/free-authentication/authentication' })
  const [loading, setLoading] = useState(true)
  const [saving, setSaving] = useState(false)
  const [showPassword, setShowPassword] = useState(false)
  const [successMsg, setSuccessMsg] = useState('')
  const [errorMsg, setErrorMsg] = useState('')

  useEffect(() => { loadConfig() }, [])

  const loadConfig = async () => {
    setLoading(true)
    try {
      const res = await jiraApi.get()
      const cfg = res.data || {}
      setConfig(cfg)
      if (cfg.username) {
        setForm({ username: cfg.username, password: '', base_url: cfg.base_url || 'https://jira.transwarp.io', login_url: cfg.login_url || 'https://erp.transwarp.io/api/v1/free-authentication/authentication' })
      }
    } catch (e) { setErrorMsg('加载配置失败: ' + (e.message || '未知错误')) }
    finally { setLoading(false) }
  }

  const handleSave = async () => {
    if (!form.username || !form.password) { setErrorMsg('请填写用户名和密码'); return }
    setSaving(true); setErrorMsg(''); setSuccessMsg('')
    try {
      const res = await jiraApi.save(form)
      setConfig(res.data)
      setForm({ username: res.data.username, password: '', base_url: res.data.base_url || 'https://jira.transwarp.io', login_url: res.data.login_url || 'https://erp.transwarp.io/api/v1/free-authentication/authentication' })
      setSuccessMsg('Jira 配置已保存，凭据验证通过')
      setTimeout(() => setSuccessMsg(''), 3000)
    } catch (e) { setErrorMsg('保存失败: ' + (e.message || '未知错误')) }
    finally { setSaving(false) }
  }

  const inputStyle = { width: '100%', padding: '10px 14px', border: '1px solid var(--border)', borderRadius: 'var(--radius)', fontSize: 14, outline: 'none', boxSizing: 'border-box', background: '#fff', color: 'var(--text)' }
  const labelStyle = { fontSize: 13, fontWeight: 600, color: 'var(--text)', marginBottom: 6, display: 'block' }

  if (loading) return <div style={{ textAlign: 'center', padding: 48 }}><div className="loading-spinner" /></div>

  return (
    <div>
      <p style={{ fontSize: 14, color: 'var(--text-secondary)', marginBottom: 16 }}>配置 Jira 登录凭据后，AI 汇总 Patch 邮件时会自动查询 WARP 工单详情。密码使用 AES-256-GCM 加密存储。</p>

      {successMsg && <AlertBox kind="success" text={successMsg} onClose={() => setSuccessMsg('')} />}
      {errorMsg && <AlertBox kind="error" text={errorMsg} onClose={() => setErrorMsg('')} />}

      <div className="card" style={{ padding: '24px 28px', maxWidth: 600 }}>
        <div style={{ display: 'flex', flexDirection: 'column', gap: 16 }}>
          <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: 16 }}>
            <div><label style={labelStyle}>JIRA 地址</label><input value={form.base_url} onChange={e => setForm({ ...form, base_url: e.target.value })} placeholder="https://jira.transwarp.io" style={inputStyle} /></div>
            <div><label style={labelStyle}>SSO 登录地址</label><input value={form.login_url} onChange={e => setForm({ ...form, login_url: e.target.value })} placeholder="https://erp.transwarp.io/..." style={inputStyle} /></div>
          </div>
          <div><label style={labelStyle}>用户名</label><input value={form.username} onChange={e => setForm({ ...form, username: e.target.value })} placeholder="输入 SSO 用户名" style={inputStyle} /></div>
          <div>
            <label style={labelStyle}>密码</label>
            <div style={{ position: 'relative' }}>
              <input type={showPassword ? 'text' : 'password'} value={form.password} onChange={e => setForm({ ...form, password: e.target.value })} placeholder={config?.username ? '留空则保持原密码不变' : '输入密码'} style={{ ...inputStyle, paddingRight: 40 }} />
              <button onClick={() => setShowPassword(!showPassword)} style={{ position: 'absolute', right: 10, top: '50%', transform: 'translateY(-50%)', padding: 4, border: 'none', background: 'none', cursor: 'pointer', color: 'var(--text-muted)' }}>
                {showPassword ? <EyeOff size={16} /> : <Eye size={16} />}
              </button>
            </div>
            {config?.username && <span style={{ fontSize: 11, color: 'var(--text-muted)', marginTop: 4, display: 'block' }}>当前用户：{config.username}，如不修改密码请留空</span>}
          </div>
          <button onClick={handleSave} disabled={saving} style={{ display: 'inline-flex', alignItems: 'center', gap: 6, padding: '10px 24px', border: 'none', borderRadius: 'var(--radius)', background: '#6366F1', color: '#fff', cursor: saving ? 'not-allowed' : 'pointer', fontSize: 14, fontWeight: 500, opacity: saving ? 0.7 : 1, alignSelf: 'flex-start' }}>
            {saving ? '验证并保存中...' : '保存配置'}
          </button>
        </div>
        {config?.username && (
          <div style={{ marginTop: 20, padding: '12px 16px', background: '#F0FDF4', border: '1px solid #BBF7D0', borderRadius: 'var(--radius)', display: 'flex', alignItems: 'center', gap: 8, fontSize: 13, color: '#166534' }}>
            <CheckCircle size={16} /><span>已配置 Jira 凭据（用户：{config.username}）</span>
          </div>
        )}
      </div>
    </div>
  )
}

/* ========== AI 配置 ========== */

function AISection() {
  const [configs, setConfigs] = useState([])
  const [editing, setEditing] = useState(null)
  const [form, setForm] = useState({ name: '', endpoint: '', api_key: '', model: '', is_default: false })
  const [saving, setSaving] = useState(false)
  const [aiPrompt, setAiPrompt] = useState('')
  const [promptLoaded, setPromptLoaded] = useState(false)
  const [promptSaving, setPromptSaving] = useState(false)
  const [promptSaved, setPromptSaved] = useState(false)

  useEffect(() => { loadConfigs() }, [])

  const loadConfigs = async () => {
    try { const res = await aiApi.listConfigs(); setConfigs(res.data || []) } catch (e) { console.error(e) }
  }

  useEffect(() => {
    if (!promptLoaded) {
      aiApi.getPrompt().then(res => { setAiPrompt(res.data?.prompt || ''); setPromptLoaded(true) }).catch(() => setPromptLoaded(true))
    }
  }, [promptLoaded])

  const savePrompt = async () => {
    setPromptSaving(true)
    try { await aiApi.savePrompt(aiPrompt); setPromptSaved(true); setTimeout(() => setPromptSaved(false), 2000) }
    catch (e) { alert('保存提示词失败: ' + (e.message || '未知错误')) }
    finally { setPromptSaving(false) }
  }

  const resetPrompt = () => {
    setAiPrompt(`你是一个专业的软件 Patch 分析助手。请根据以下 Patch 发布通知邮件内容，生成一份结构化的 Patch 调整摘要。

如果邮件正文中包含 WARP-xxxxx 格式的工单编号，请使用 query_warp_issue 工具查询该工单在 JIRA 中的详细信息（标题、描述、状态、评论等），结合 JIRA 工单内容更准确地分析 Patch 调整的原因和影响。

请按以下格式输出：

## Patch 基本信息
- 产品及版本
- Patch 类型（预览/通用/定向）
- Patch 日期

## 调整内容
列出本次 Patch 涉及的主要调整和修复内容

## 影响范围
分析本次 Patch 可能影响的模块和功能

## 注意事项
部署或升级时需要注意的事项

---

邮件内容：
`)
  }

  const startEdit = (cfg = null) => {
    if (cfg) { setEditing(cfg.id); setForm({ name: cfg.name, endpoint: cfg.endpoint, api_key: cfg.api_key, model: cfg.model, is_default: cfg.is_default }) }
    else { setEditing('new'); setForm({ name: '', endpoint: '', api_key: '', model: '', is_default: configs.length === 0 }) }
  }

  const handleSave = async () => {
    if (!form.name || !form.endpoint || !form.api_key || !form.model) { alert('请填写所有必填项'); return }
    setSaving(true)
    try {
      if (editing === 'new') await aiApi.createConfig(form)
      else await aiApi.updateConfig(editing, form)
      setEditing(null); loadConfigs()
    } catch (e) { alert('保存失败: ' + (e.message || '未知错误')) }
    finally { setSaving(false) }
  }

  const handleDelete = async (id) => {
    if (!confirm('确定删除此 AI 配置？')) return
    try { await aiApi.deleteConfig(id); loadConfigs() } catch (e) { alert('删除失败: ' + (e.message || '未知错误')) }
  }

  const handleSetDefault = async (id) => {
    try { await aiApi.setDefault(id); loadConfigs() } catch (e) { alert('设置失败: ' + (e.message || '未知错误')) }
  }

  const inputStyle = { width: '100%', padding: '8px 12px', border: '1px solid var(--border)', borderRadius: 'var(--radius)', fontSize: 13, outline: 'none', boxSizing: 'border-box' }

  return (
    <div>
      <p style={{ fontSize: 14, color: 'var(--text-secondary)', marginBottom: 16 }}>配置 AI 服务后，可在 Patch 汇总页面点击"AI"按钮对 Patch 邮件进行智能分析。支持 OpenAI 兼容接口。</p>

      {/* 已有配置列表 */}
      {configs.length > 0 && (
        <div style={{ display: 'flex', flexDirection: 'column', gap: 12, marginBottom: 16 }}>
          {configs.map(cfg => (
            <div key={cfg.id} className="card" style={{ padding: 16, borderColor: cfg.is_default ? '#C4B5FD' : undefined, background: cfg.is_default ? '#F5F3FF' : undefined }}>
              <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between' }}>
                <div>
                  <div style={{ display: 'flex', alignItems: 'center', gap: 8 }}>
                    <span style={{ fontWeight: 600, fontSize: 14 }}>{cfg.name}</span>
                    {cfg.is_default && <span style={{ fontSize: 11, background: '#7C3AED', color: '#fff', padding: '1px 6px', borderRadius: 4 }}>默认</span>}
                  </div>
                  <div style={{ fontSize: 12, color: 'var(--text-secondary)', marginTop: 4 }}>{cfg.endpoint} · {cfg.model}</div>
                </div>
                <div style={{ display: 'flex', gap: 6 }}>
                  {!cfg.is_default && <button onClick={() => handleSetDefault(cfg.id)} title="设为默认" className="btn btn-secondary btn-sm"><Star size={12} /> 设为默认</button>}
                  <button onClick={() => startEdit(cfg)} className="btn btn-secondary btn-sm">编辑</button>
                  <button onClick={() => handleDelete(cfg.id)} className="btn btn-danger btn-sm"><Trash2 size={12} /> 删除</button>
                </div>
              </div>
            </div>
          ))}
        </div>
      )}

      {/* 添加/编辑表单 */}
      {editing ? (
        <div className="card" style={{ padding: 20, borderColor: '#C4B5FD', background: '#FAFAFE' }}>
          <h4 style={{ margin: '0 0 16px', fontSize: 14, fontWeight: 600, color: '#5B21B6' }}>{editing === 'new' ? '添加 AI 配置' : '编辑配置'}</h4>
          <div style={{ display: 'flex', flexDirection: 'column', gap: 12, maxWidth: 520 }}>
            <div><label style={{ fontSize: 12, fontWeight: 500, color: 'var(--text-secondary)', marginBottom: 4, display: 'block' }}>配置名称 *</label><input value={form.name} onChange={e => setForm({ ...form, name: e.target.value })} placeholder="如：DeepSeek" style={inputStyle} /></div>
            <div><label style={{ fontSize: 12, fontWeight: 500, color: 'var(--text-secondary)', marginBottom: 4, display: 'block' }}>API Endpoint *</label><input value={form.endpoint} onChange={e => setForm({ ...form, endpoint: e.target.value })} placeholder="https://api.deepseek.com/v1" style={inputStyle} /></div>
            <div><label style={{ fontSize: 12, fontWeight: 500, color: 'var(--text-secondary)', marginBottom: 4, display: 'block' }}>API Key *</label><input type="password" value={form.api_key} onChange={e => setForm({ ...form, api_key: e.target.value })} placeholder="sk-..." style={inputStyle} /></div>
            <div><label style={{ fontSize: 12, fontWeight: 500, color: 'var(--text-secondary)', marginBottom: 4, display: 'block' }}>模型名称 *</label><input value={form.model} onChange={e => setForm({ ...form, model: e.target.value })} placeholder="deepseek-chat / qwen-plus" style={inputStyle} /></div>
            <label style={{ display: 'flex', alignItems: 'center', gap: 8, fontSize: 13 }}><input type="checkbox" checked={form.is_default} onChange={e => setForm({ ...form, is_default: e.target.checked })} /> 设为默认配置</label>
            <div style={{ display: 'flex', gap: 8, justifyContent: 'flex-end', marginTop: 4 }}>
              <button onClick={() => setEditing(null)} className="btn btn-secondary">取消</button>
              <button onClick={handleSave} disabled={saving} className="btn btn-primary">{saving ? '保存中...' : '保存'}</button>
            </div>
          </div>
        </div>
      ) : (
        <button onClick={() => startEdit()} className="btn btn-secondary" style={{ borderStyle: 'dashed', width: '100%', justifyContent: 'center', padding: '10px 16px' }}><Plus size={14} /> 添加 AI 配置</button>
      )}

      {/* 提示词编辑区 */}
      <div style={{ marginTop: 24, padding: 16, border: '1px solid #E0E7FF', borderRadius: 'var(--radius)', background: '#F8F9FF' }}>
        <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', marginBottom: 10 }}>
          <div style={{ display: 'flex', alignItems: 'center', gap: 6 }}>
            <Edit3 size={14} color="#7C3AED" />
            <span style={{ fontSize: 13, fontWeight: 600, color: '#5B21B6' }}>AI 汇总提示词</span>
          </div>
          <div style={{ display: 'flex', gap: 6 }}>
            <button onClick={resetPrompt} className="btn btn-secondary btn-sm">恢复默认</button>
            <button onClick={savePrompt} disabled={promptSaving} className="btn btn-primary btn-sm" style={{ background: '#7C3AED' }}>
              {promptSaving ? '保存中...' : (promptSaved ? '已保存 ✓' : '保存提示词')}
            </button>
          </div>
        </div>
        <textarea value={aiPrompt} onChange={e => setAiPrompt(e.target.value)} placeholder="输入 AI 汇总时使用的提示词..."
          style={{ width: '100%', minHeight: 140, padding: '10px 12px', border: '1px solid #C7D2FE', borderRadius: 'var(--radius)', fontSize: 12, lineHeight: 1.6, color: 'var(--text)', background: '#fff', outline: 'none', resize: 'vertical', boxSinging: 'border-box', fontFamily: 'inherit' }} />
        <div style={{ fontSize: 11, color: 'var(--text-muted)', marginTop: 6 }}>提示词末尾会自动拼接邮件正文。留空则使用默认提示词。</div>
      </div>
    </div>
  )
}

/* ========== 工具组件 ========== */

function AlertBox({ kind, text, onClose }) {
  const isSuccess = kind === 'success'
  return (
    <div style={{ marginBottom: 16, padding: '10px 14px', background: isSuccess ? '#ECFDF5' : '#FEF2F2', border: `1px solid ${isSuccess ? '#A7F3D0' : '#FECACA'}`, borderRadius: 'var(--radius)', display: 'flex', alignItems: 'center', gap: 8, fontSize: 13, color: isSuccess ? '#065F46' : '#DC2626' }}>
      {isSuccess ? <CheckCircle size={14} /> : <AlertCircle size={14} />}
      <span style={{ flex: 1 }}>{text}</span>
      <button onClick={onClose} style={{ padding: 2, border: 'none', background: 'none', cursor: 'pointer', color: 'inherit' }}><X size={14} /></button>
    </div>
  )
}
