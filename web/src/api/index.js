const API_BASE = '/api';

async function request(url, options = {}) {
  const res = await fetch(API_BASE + url, {
    headers: { 'Content-Type': 'application/json', ...options.headers },
    ...options,
  });
  const data = await res.json();
  if (!res.ok) throw new Error(data.error || '请求失败');
  return data;
}

// 账户相关
export const accountApi = {
  list: () => request('/accounts'),
  get: (id) => request(`/accounts/${id}`),
  create: (data) => request('/accounts', { method: 'POST', body: JSON.stringify(data) }),
  update: (id, data) => request(`/accounts/${id}`, { method: 'PUT', body: JSON.stringify(data) }),
  delete: (id) => request(`/accounts/${id}`, { method: 'DELETE' }),
  test: (id) => request(`/accounts/${id}/test`, { method: 'POST' }),
};

// 邮件相关
export const mailApi = {
  summary: () => request('/mails/summary'),
  list: (accountId, params = {}) => {
    const query = new URLSearchParams(params).toString();
    return request(`/mails/account/${accountId}${query ? '?' + query : ''}`);
  },
  detail: (id) => request(`/mails/${id}`),
  sync: (accountId, days = 30) => request(`/mails/sync/${accountId}?days=${days}`, { method: 'POST' }),
};

// 统计
export const statsApi = {
  overview: () => request('/stats/overview'),
};

// Patch 汇总
export const patchApi = {
  summary: (params = {}) => {
    const query = new URLSearchParams(params).toString();
    return request(`/patches/summary${query ? '?' + query : ''}`);
  },
};

// JIRA 配置
export const jiraApi = {
  get: () => request('/jira-config'),
  save: (data) => request('/jira-config', { method: 'POST', body: JSON.stringify(data) }),
};

// 初始化配置状态
export const setupApi = {
  getStatus: () => request('/setup/status'),
  complete: () => request('/setup/complete', { method: 'POST' }),
};

// AI 配置和汇总
export const aiApi = {
  listConfigs: () => request('/ai/configs'),
  createConfig: (data) => request('/ai/configs', { method: 'POST', body: JSON.stringify(data) }),
  updateConfig: (id, data) => request(`/ai/configs/${id}`, { method: 'PUT', body: JSON.stringify(data) }),
  deleteConfig: (id) => request(`/ai/configs/${id}`, { method: 'DELETE' }),
  setDefault: (id) => request(`/ai/configs/${id}/default`, { method: 'PUT' }),
  summarize: (mailId, { prompt = '', force = false } = {}) => request('/ai/summarize', {
    method: 'POST',
    body: JSON.stringify({ mail_id: mailId, prompt, force }),
  }),
  getSummary: (mailId) => request(`/ai/summary/${mailId}`),
  batchSummaries: (mailIds) => request('/ai/summaries/batch', {
    method: 'POST',
    body: JSON.stringify({ mail_ids: mailIds }),
  }),
  getPrompt: () => request('/ai/prompt'),
  savePrompt: (prompt) => request('/ai/prompt', { method: 'PUT', body: JSON.stringify({ prompt }) }),
};
