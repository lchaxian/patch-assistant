import { useState } from 'react'
import { accountApi, jiraApi, setupApi } from '../api'
import { Mail, Eye, EyeOff, ArrowRight, CheckCircle, SkipForward } from 'lucide-react'

const DEFAULT_IMAP = { host: 'imap.exmail.qq.com', port: 993 }

export default function SetupWizard({ onComplete }) {
  const [step, setStep] = useState(1) // 1: 邮箱, 2: Jira
  const [showPassword, setShowPassword] = useState(false)

  // 邮箱表单
  const [emailForm, setEmailForm] = useState({
    email: '',
    display_name: '',
    password: '',
    imap_host: DEFAULT_IMAP.host,
    imap_port: DEFAULT_IMAP.port,
    use_tls: true,
  })
  const [emailSaving, setEmailSaving] = useState(false)
  const [emailError, setEmailError] = useState('')

  // Jira 表单
  const [jiraForm, setJiraForm] = useState({
    username: '',
    password: '',
    base_url: 'https://jira.transwarp.io',
    login_url: 'https://erp.transwarp.io/api/v1/free-authentication/authentication',
  })
  const [jiraSaving, setJiraSaving] = useState(false)
  const [jiraError, setJiraError] = useState('')

  const handleSaveEmail = async (e) => {
    e.preventDefault()
    if (!emailForm.email || !emailForm.password) {
      setEmailError('请填写邮箱地址和密码')
      return
    }
    setEmailSaving(true)
    setEmailError('')
    try {
      await accountApi.create(emailForm)
      setStep(2)
    } catch (err) {
      setEmailError('创建失败: ' + (err.message || '未知错误'))
    } finally {
      setEmailSaving(false)
    }
  }

  const handleSaveJira = async (e) => {
    e.preventDefault()
    if (!jiraForm.username || !jiraForm.password) {
      setJiraError('请填写用户名和密码')
      return
    }
    setJiraSaving(true)
    setJiraError('')
    try {
      await jiraApi.save(jiraForm)
      try { await setupApi.complete() } catch (e) { /* ignore */ }
      await onComplete()
    } catch (err) {
      setJiraError('保存失败: ' + (err.message || '未知错误'))
    } finally {
      setJiraSaving(false)
    }
  }

  const handleSkipEmail = async () => {
    setStep(2)
  }

  const handleSkipAll = async () => {
    try { await setupApi.complete() } catch (e) { /* ignore */ }
    await onComplete()
  }

  const handleSkipJira = async () => {
    try { await setupApi.complete() } catch (e) { /* ignore */ }
    await onComplete()
  }

  const inputStyle = {
    width: '100%',
    padding: '10px 14px',
    border: '1px solid var(--border)',
    borderRadius: 'var(--radius)',
    fontSize: 14,
    outline: 'none',
    boxSizing: 'border-box',
    transition: 'border-color 0.15s',
    background: '#fff',
    color: 'var(--text)',
  }

  const labelStyle = {
    fontSize: 13,
    fontWeight: 600,
    color: 'var(--text)',
    marginBottom: 6,
    display: 'block',
  }

  return (
    <div style={{
      minHeight: '100vh',
      display: 'flex',
      alignItems: 'center',
      justifyContent: 'center',
      background: 'linear-gradient(135deg, #667eea 0%, #764ba2 100%)',
      padding: 24,
    }}>
      <div style={{
        width: '100%',
        maxWidth: 520,
        background: '#fff',
        borderRadius: 16,
        boxShadow: '0 20px 60px rgba(0,0,0,0.15)',
        overflow: 'hidden',
      }}>
        {/* Header */}
        <div style={{
          padding: '32px 32px 24px',
          background: 'linear-gradient(135deg, #6366F1 0%, #8B5CF6 100%)',
          color: '#fff',
          textAlign: 'center',
        }}>
          <div style={{
            width: 56,
            height: 56,
            borderRadius: 16,
            background: 'rgba(255,255,255,0.2)',
            display: 'flex',
            alignItems: 'center',
            justifyContent: 'center',
            margin: '0 auto 16px',
          }}>
            <Mail size={28} />
          </div>
          <h1 style={{ margin: 0, fontSize: 24, fontWeight: 700 }}>欢迎使用 Patch助手</h1>
          <p style={{ margin: '8px 0 0', fontSize: 14, opacity: 0.9 }}>按需配置，均可跳过，稍后可在设置中补充</p>
        </div>

        {/* Steps indicator */}
        <div style={{ display: 'flex', padding: '20px 32px 0', gap: 8 }}>
          <StepIndicator number={1} label="邮箱配置" active={step >= 1} completed={step > 1} />
          <StepIndicator number={2} label="Jira 配置" active={step >= 2} completed={false} />
        </div>

        {/* Step 1: Email */}
        {step === 1 && (
          <form onSubmit={handleSaveEmail} style={{ padding: '24px 32px 32px' }}>
            <p style={{ fontSize: 14, color: 'var(--text-secondary)', margin: '0 0 20px', lineHeight: 1.6 }}>
              添加一个邮箱账户，用于同步 Patch 发布通知邮件。可跳过，稍后在设置中添加。
            </p>

            {emailError && (
              <div style={{
                marginBottom: 16, padding: '10px 14px', background: '#FEF2F2',
                border: '1px solid #FECACA', borderRadius: 'var(--radius)',
                color: '#DC2626', fontSize: 13,
              }}>
                {emailError}
              </div>
            )}

            <div style={{ display: 'flex', flexDirection: 'column', gap: 14 }}>
              <div>
                <label style={labelStyle}>邮箱地址 *</label>
                <input
                  type="email"
                  value={emailForm.email}
                  onChange={e => setEmailForm({ ...emailForm, email: e.target.value })}
                  placeholder="user@company.com"
                  style={inputStyle}
                  required
                />
              </div>

              <div>
                <label style={labelStyle}>显示名称</label>
                <input
                  value={emailForm.display_name}
                  onChange={e => setEmailForm({ ...emailForm, display_name: e.target.value })}
                  placeholder="可选，方便识别"
                  style={inputStyle}
                />
              </div>

              <div>
                <label style={labelStyle}>邮箱密码 *</label>
                <div style={{ position: 'relative' }}>
                  <input
                    type={showPassword ? 'text' : 'password'}
                    value={emailForm.password}
                    onChange={e => setEmailForm({ ...emailForm, password: e.target.value })}
                    placeholder="输入邮箱密码"
                    style={{ ...inputStyle, paddingRight: 40 }}
                    required
                  />
                  <button
                    type="button"
                    onClick={() => setShowPassword(!showPassword)}
                    style={{
                      position: 'absolute', right: 10, top: '50%', transform: 'translateY(-50%)',
                      padding: 4, border: 'none', background: 'none', cursor: 'pointer', color: 'var(--text-muted)',
                    }}
                  >
                    {showPassword ? <EyeOff size={16} /> : <Eye size={16} />}
                  </button>
                </div>
              </div>

              <div style={{ display: 'grid', gridTemplateColumns: '2fr 1fr', gap: 12 }}>
                <div>
                  <label style={labelStyle}>IMAP 服务器</label>
                  <input
                    value={emailForm.imap_host}
                    onChange={e => setEmailForm({ ...emailForm, imap_host: e.target.value })}
                    style={inputStyle}
                  />
                </div>
                <div>
                  <label style={labelStyle}>端口</label>
                  <input
                    type="number"
                    value={emailForm.imap_port}
                    onChange={e => setEmailForm({ ...emailForm, imap_port: parseInt(e.target.value) || 993 })}
                    style={inputStyle}
                  />
                </div>
              </div>

              <label style={{ display: 'flex', alignItems: 'center', gap: 8, fontWeight: 400, fontSize: 13 }}>
                <input
                  type="checkbox"
                  checked={emailForm.use_tls}
                  onChange={e => setEmailForm({ ...emailForm, use_tls: e.target.checked })}
                />
                使用 SSL/TLS 加密连接
              </label>
            </div>

            <div style={{ display: 'flex', gap: 8, marginTop: 24 }}>
              <button
                type="submit"
                disabled={emailSaving}
                style={{
                  display: 'inline-flex', alignItems: 'center', gap: 6,
                  padding: '10px 24px', border: 'none', borderRadius: 'var(--radius)',
                  background: '#6366F1', color: '#fff', cursor: emailSaving ? 'not-allowed' : 'pointer',
                  fontSize: 14, fontWeight: 500, opacity: emailSaving ? 0.7 : 1,
                  transition: 'all 0.15s', flex: 1,
                }}
              >
                {emailSaving ? '保存中...' : '下一步'}
                {!emailSaving && <ArrowRight size={16} />}
              </button>
              <button
                type="button"
                onClick={handleSkipEmail}
                style={{
                  display: 'inline-flex', alignItems: 'center', gap: 6,
                  padding: '10px 16px', border: '1px solid var(--border)', borderRadius: 'var(--radius)',
                  background: '#fff', color: 'var(--text-secondary)', cursor: 'pointer',
                  fontSize: 13, transition: 'all 0.15s',
                }}
              >
                <SkipForward size={14} /> 跳过此步
              </button>
              <button
                type="button"
                onClick={handleSkipAll}
                style={{
                  display: 'inline-flex', alignItems: 'center', gap: 6,
                  padding: '10px 16px', border: '1px solid var(--border)', borderRadius: 'var(--radius)',
                  background: '#fff', color: 'var(--text-muted)', cursor: 'pointer',
                  fontSize: 12, transition: 'all 0.15s',
                }}
              >
                跳过全部
              </button>
            </div>
          </form>
        )}

        {/* Step 2: Jira */}
        {step === 2 && (
          <form onSubmit={handleSaveJira} style={{ padding: '24px 32px 32px' }}>
            <p style={{ fontSize: 14, color: 'var(--text-secondary)', margin: '0 0 20px', lineHeight: 1.6 }}>
              配置 Jira 登录凭据后，AI 汇总时可自动查询 WARP 工单详情，生成更准确的 Patch 分析。可跳过，稍后在设置中配置。密码使用 AES-256-GCM 加密存储。
            </p>

            {jiraError && (
              <div style={{
                marginBottom: 16, padding: '10px 14px', background: '#FEF2F2',
                border: '1px solid #FECACA', borderRadius: 'var(--radius)',
                color: '#DC2626', fontSize: 13,
              }}>
                {jiraError}
              </div>
            )}

            <div style={{ display: 'flex', flexDirection: 'column', gap: 14 }}>
              <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: 12 }}>
                <div>
                  <label style={labelStyle}>JIRA 地址</label>
                  <input
                    value={jiraForm.base_url}
                    onChange={e => setJiraForm({ ...jiraForm, base_url: e.target.value })}
                    placeholder="https://jira.transwarp.io"
                    style={inputStyle}
                  />
                </div>
                <div>
                  <label style={labelStyle}>SSO 登录地址</label>
                  <input
                    value={jiraForm.login_url}
                    onChange={e => setJiraForm({ ...jiraForm, login_url: e.target.value })}
                    placeholder="https://erp.transwarp.io/..."
                    style={inputStyle}
                  />
                </div>
              </div>

              <div>
                <label style={labelStyle}>用户名 *</label>
                <input
                  value={jiraForm.username}
                  onChange={e => setJiraForm({ ...jiraForm, username: e.target.value })}
                  placeholder="输入 SSO 用户名"
                  style={inputStyle}
                  required
                />
              </div>

              <div>
                <label style={labelStyle}>密码 *</label>
                <div style={{ position: 'relative' }}>
                  <input
                    type={showPassword ? 'text' : 'password'}
                    value={jiraForm.password}
                    onChange={e => setJiraForm({ ...jiraForm, password: e.target.value })}
                    placeholder="输入 SSO 密码"
                    style={{ ...inputStyle, paddingRight: 40 }}
                    required
                  />
                  <button
                    type="button"
                    onClick={() => setShowPassword(!showPassword)}
                    style={{
                      position: 'absolute', right: 10, top: '50%', transform: 'translateY(-50%)',
                      padding: 4, border: 'none', background: 'none', cursor: 'pointer', color: 'var(--text-muted)',
                    }}
                  >
                    {showPassword ? <EyeOff size={16} /> : <Eye size={16} />}
                  </button>
                </div>
              </div>
            </div>

            <div style={{ display: 'flex', gap: 8, marginTop: 24 }}>
              <button
                type="submit"
                disabled={jiraSaving}
                style={{
                  display: 'inline-flex', alignItems: 'center', gap: 6,
                  padding: '10px 24px', border: 'none', borderRadius: 'var(--radius)',
                  background: '#6366F1', color: '#fff', cursor: jiraSaving ? 'not-allowed' : 'pointer',
                  fontSize: 14, fontWeight: 500, opacity: jiraSaving ? 0.7 : 1,
                  transition: 'all 0.15s', flex: 1,
                }}
              >
                {jiraSaving ? '保存中...' : '完成配置'}
                {!jiraSaving && <CheckCircle size={16} />}
              </button>
              <button
                type="button"
                onClick={handleSkipJira}
                style={{
                  display: 'inline-flex', alignItems: 'center', gap: 6,
                  padding: '10px 16px', border: '1px solid var(--border)', borderRadius: 'var(--radius)',
                  background: '#fff', color: 'var(--text-secondary)', cursor: 'pointer',
                  fontSize: 13, transition: 'all 0.15s',
                }}
              >
                <SkipForward size={14} /> 跳过，稍后配置
              </button>
            </div>
          </form>
        )}
      </div>
    </div>
  )
}

function StepIndicator({ number, label, active, completed }) {
  return (
    <div style={{
      display: 'flex', alignItems: 'center', gap: 8, flex: 1,
      padding: '8px 12px', borderRadius: 8,
      background: completed ? '#ECFDF5' : active ? '#EEF2FF' : '#F8FAFC',
      border: `1px solid ${completed ? '#A7F3D0' : active ? '#C7D2FE' : '#E2E8F0'}`,
    }}>
      <div style={{
        width: 24, height: 24, borderRadius: '50%',
        display: 'flex', alignItems: 'center', justifyContent: 'center',
        fontSize: 12, fontWeight: 700,
        background: completed ? '#059669' : active ? '#6366F1' : '#CBD5E1',
        color: '#fff',
      }}>
        {completed ? <CheckCircle size={14} /> : number}
      </div>
      <span style={{
        fontSize: 13, fontWeight: 500,
        color: completed ? '#065F46' : active ? '#4338CA' : '#94A3B8',
      }}>
        {label}
      </span>
    </div>
  )
}
