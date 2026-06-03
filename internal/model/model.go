package model

import "time"

// Account 邮箱账户
type Account struct {
	ID           int64      `json:"id"`
	Email        string     `json:"email"`
	DisplayName  string     `json:"display_name"`
	IMAPHost     string     `json:"imap_host"`
	IMAPPort     int        `json:"imap_port"`
	EncryptedPwd string     `json:"-"`
	Password     string     `json:"password,omitempty"`
	UseTLS       bool       `json:"use_tls"`
	Status       string     `json:"status"`
	LastError    string     `json:"last_error,omitempty"`
	LastSyncAt   *time.Time `json:"last_sync_at,omitempty"`
	CreatedAt    time.Time  `json:"created_at"`
	UpdatedAt    time.Time  `json:"updated_at"`
}

// MailItem 邮件摘要
type MailItem struct {
	ID        int64     `json:"id"`
	AccountID int64     `json:"account_id"`
	MessageID string    `json:"message_id"`
	From      string    `json:"from_addr"`
	FromName  string    `json:"from_name"`
	To        string    `json:"to_addr"`
	Subject   string    `json:"subject"`
	Date      time.Time `json:"date"`
	Size      int64     `json:"size"`
	IsRead    bool      `json:"is_read"`
	HasAttach bool      `json:"has_attach"`
	Folder    string    `json:"folder"`
	BodyText  string    `json:"body_text,omitempty"`
	BodyHTML  string    `json:"body_html,omitempty"`
	CreatedAt time.Time `json:"created_at"`
}

// OverviewStats 总览统计
type OverviewStats struct {
	TotalAccounts  int64 `json:"total_accounts"`
	ActiveAccounts int64 `json:"active_accounts"`
	TotalMails     int64 `json:"total_mails"`
	UnreadMails    int64 `json:"unread_mails"`
	TodayMails     int64 `json:"today_mails"`
	WeekMails      int64 `json:"week_mails"`
}

// MailSummaryPerAccount 每账户邮件统计
type MailSummaryPerAccount struct {
	AccountID   int64  `json:"account_id"`
	Email       string `json:"email"`
	DisplayName string `json:"display_name"`
	TotalMails  int64  `json:"total_mails"`
	UnreadMails int64  `json:"unread_mails"`
	TodayMails  int64  `json:"today_mails"`
	LastSyncAt  string `json:"last_sync_at"`
}

// SyncResult 同步结果
type SyncResult struct {
	AccountID  int64  `json:"account_id"`
	NewMails   int    `json:"new_mails"`
	TotalMails int    `json:"total_mails"`
	Error      string `json:"error,omitempty"`
}

// PatchInfo 从 Patch 发布通知邮件标题中解析出的信息
type PatchInfo struct {
	MailID    int64     `json:"mail_id"`
	AccountID int64     `json:"account_id"`
	Subject   string    `json:"subject"`
	Type      string    `json:"type"`
	Product   string    `json:"product"`
	Version   string    `json:"version"`
	Date      string    `json:"patch_date"`
	Seq       string    `json:"seq"`
	MailDate  time.Time `json:"mail_date"`
	FromName  string    `json:"from_name"`
	FromAddr  string    `json:"from_addr"`
}

// PatchSummaryResponse Patch 汇总响应
type PatchSummaryResponse struct {
	Range      string         `json:"range"`
	TotalCount int            `json:"total_count"`
	Patches    []PatchInfo    `json:"patches"`
	ByProduct  map[string]int `json:"by_product"`
	ByType     map[string]int `json:"by_type"`
	SyncResult *SyncResult    `json:"sync_result,omitempty"`
}

// AIConfig AI 服务配置
type AIConfig struct {
	ID        int64     `json:"id"`
	Name      string    `json:"name"`
	Endpoint  string    `json:"endpoint"`
	APIKey    string    `json:"api_key"`
	Model     string    `json:"model"`
	IsDefault bool      `json:"is_default"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// AISummarizeRequest AI 汇总请求
type AISummarizeRequest struct {
	MailID int64  `json:"mail_id" binding:"required"`
	Prompt string `json:"prompt,omitempty"`
	Force  bool   `json:"force,omitempty"`
}

// JiraConfig JIRA 系统配置（保留向后兼容）
type JiraConfig struct {
	ID        int64     `json:"id"`
	Username  string    `json:"username"`
	Password  string    `json:"password"`
	BaseURL   string    `json:"base_url"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// SSOConfig JIRA 登录配置
type SSOConfig struct {
	ID        int64     `json:"id"`
	Username  string    `json:"username"`
	Password  string    `json:"password"`
	BaseURL   string    `json:"base_url"`
	LoginURL  string    `json:"login_url"`
	WikiURL   string    `json:"wiki_url"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// JiraLink JIRA 工单链接
type JiraLink struct {
	Key string `json:"key"`
	URL string `json:"url"`
}

// WikiLink Wiki 文档链接
type WikiLink struct {
	ID    string `json:"id"`
	Title string `json:"title"`
	URL   string `json:"url"`
}

// AISummarizeResponse AI 汇总响应
type AISummarizeResponse struct {
	MailID     int64      `json:"mail_id"`
	Subject    string     `json:"subject"`
	Summary    string     `json:"summary"`
	Provider   string     `json:"provider"`
	Model      string     `json:"model"`
	JiraLinks  []JiraLink `json:"jira_links"`
	WikiLinks  []WikiLink `json:"wiki_links"`
	CreatedAt  string     `json:"created_at,omitempty"`
}

// AISummary AI 汇总持久化记录
type AISummary struct {
	ID         int64       `json:"id"`
	MailID     int64       `json:"mail_id"`
	Subject    string      `json:"subject"`
	Summary    string      `json:"summary"`
	Provider   string      `json:"provider"`
	Model      string      `json:"model"`
	JiraLinks  []JiraLink  `json:"jira_links"`
	WikiLinks  []WikiLink  `json:"wiki_links"`
	CreatedAt  time.Time   `json:"created_at"`
	UpdatedAt  time.Time   `json:"updated_at"`
}
