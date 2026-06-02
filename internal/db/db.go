package db

import (
	"database/sql"
	"fmt"
	"time"

	_ "modernc.org/sqlite"

	"github.com/lchaxian/patch-assistant/internal/model"
)

var (
	DB *sql.DB
)

// Init 初始化数据库
func Init(dbPath string) error {
	var err error
	DB, err = sql.Open("sqlite", dbPath+"?_pragma=journal_mode(WAL)&_pragma=foreign_keys(1)")
	if err != nil {
		return fmt.Errorf("open database: %w", err)
	}

	DB.SetMaxOpenConns(1)
	DB.SetMaxIdleConns(1)
	DB.SetConnMaxLifetime(0)

	if err := createTables(); err != nil {
		return fmt.Errorf("create tables: %w", err)
	}
	if err := migrateTablesV2(); err != nil {
		return fmt.Errorf("migrate tables v2: %w", err)
	}

	return nil
}

// Close 关闭数据库
func Close() {
	if DB != nil {
		DB.Close()
	}
}

func createTables() error {
	schema := `
	CREATE TABLE IF NOT EXISTS accounts (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		email TEXT NOT NULL UNIQUE,
		display_name TEXT NOT NULL DEFAULT '',
		imap_host TEXT NOT NULL DEFAULT 'imap.exmail.qq.com',
		imap_port INTEGER NOT NULL DEFAULT 993,
		encrypted_password TEXT NOT NULL,
		use_tls INTEGER NOT NULL DEFAULT 1,
		status TEXT NOT NULL DEFAULT 'active',
		last_error TEXT NOT NULL DEFAULT '',
		last_sync_at DATETIME,
		created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
		updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
	);

	CREATE TABLE IF NOT EXISTS mails (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		account_id INTEGER NOT NULL,
		message_id TEXT NOT NULL,
		from_addr TEXT NOT NULL DEFAULT '',
		from_name TEXT NOT NULL DEFAULT '',
		to_addr TEXT NOT NULL DEFAULT '',
		subject TEXT NOT NULL DEFAULT '',
		mail_date DATETIME NOT NULL,
		size INTEGER NOT NULL DEFAULT 0,
		is_read INTEGER NOT NULL DEFAULT 0,
		has_attach INTEGER NOT NULL DEFAULT 0,
		folder TEXT NOT NULL DEFAULT 'INBOX',
		body_text TEXT NOT NULL DEFAULT '',
		body_html TEXT NOT NULL DEFAULT '',
		created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
		FOREIGN KEY (account_id) REFERENCES accounts(id) ON DELETE CASCADE,
		UNIQUE(account_id, message_id, folder)
	);

	CREATE INDEX IF NOT EXISTS idx_mails_account_id ON mails(account_id);
	CREATE INDEX IF NOT EXISTS idx_mails_date ON mails(mail_date DESC);
	CREATE INDEX IF NOT EXISTS idx_mails_from ON mails(from_addr);
	CREATE INDEX IF NOT EXISTS idx_mails_is_read ON mails(is_read);
	CREATE INDEX IF NOT EXISTS idx_mails_subject ON mails(subject);

	CREATE TABLE IF NOT EXISTS patch_infos (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		mail_id INTEGER NOT NULL,
		account_id INTEGER NOT NULL,
		subject TEXT NOT NULL DEFAULT '',
		patch_type TEXT NOT NULL DEFAULT '',
		product TEXT NOT NULL DEFAULT '',
		version TEXT NOT NULL DEFAULT '',
		patch_date TEXT NOT NULL DEFAULT '',
		seq TEXT NOT NULL DEFAULT '',
		FOREIGN KEY (mail_id) REFERENCES mails(id) ON DELETE CASCADE,
		UNIQUE(mail_id)
	);

	CREATE INDEX IF NOT EXISTS idx_patch_account_id ON patch_infos(account_id);
	CREATE INDEX IF NOT EXISTS idx_patch_product ON patch_infos(product);
	CREATE INDEX IF NOT EXISTS idx_patch_type ON patch_infos(patch_type);
	CREATE INDEX IF NOT EXISTS idx_patch_date ON patch_infos(patch_date);

	CREATE TABLE IF NOT EXISTS ai_configs (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		name TEXT NOT NULL DEFAULT '',
		endpoint TEXT NOT NULL DEFAULT '',
		api_key TEXT NOT NULL DEFAULT '',
		model TEXT NOT NULL DEFAULT '',
		is_default INTEGER NOT NULL DEFAULT 0,
		created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
		updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
	);

	CREATE TABLE IF NOT EXISTS settings (
		key TEXT PRIMARY KEY,
		value TEXT NOT NULL DEFAULT '',
		updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
	);
	`
	_, err := DB.Exec(schema)
	return err
}

// migrateTablesV2 增量迁移
func migrateTablesV2() error {
	migrations := []string{
		`CREATE TABLE IF NOT EXISTS sso_config (
			id INTEGER PRIMARY KEY DEFAULT 1,
			username TEXT NOT NULL DEFAULT '',
			password TEXT NOT NULL DEFAULT '',
			base_url TEXT NOT NULL DEFAULT 'https://jira.transwarp.io',
			login_url TEXT NOT NULL DEFAULT '',
			created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
		)`,
	}
	for _, stmt := range migrations {
		if _, err := DB.Exec(stmt); err != nil {
			return fmt.Errorf("migrate: %w", err)
		}
	}
	return nil
}

// --- JIRA Config ---

// GetSSOConfig 获取 SSO/JIRA 配置
func GetSSOConfig() (*model.SSOConfig, error) {
	var cfg model.SSOConfig
	err := DB.QueryRow(`SELECT id, username, password, base_url, login_url, created_at, updated_at FROM sso_config WHERE id = 1`).Scan(
		&cfg.ID, &cfg.Username, &cfg.Password, &cfg.BaseURL, &cfg.LoginURL, &cfg.CreatedAt, &cfg.UpdatedAt)
	if err != nil {
		return nil, err
	}
	if cfg.Password != "" {
		plain, decErr := DecryptPassword(cfg.Password)
		if decErr == nil {
			cfg.Password = plain
		}
	}
	return &cfg, nil
}

// SaveSSOConfig 保存 SSO 配置
func SaveSSOConfig(cfg *model.SSOConfig) error {
	encPwd, err := EncryptPassword(cfg.Password)
	if err != nil {
		return fmt.Errorf("encrypt sso password: %w", err)
	}
	_, err = DB.Exec(`
		INSERT INTO sso_config (id, username, password, base_url, login_url, updated_at)
		VALUES (1, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			username = excluded.username,
			password = excluded.password,
			base_url = excluded.base_url,
			login_url = excluded.login_url,
			updated_at = excluded.updated_at`,
		cfg.Username, encPwd, cfg.BaseURL, cfg.LoginURL, time.Now())
	if err != nil {
		return fmt.Errorf("save sso config: %w", err)
	}
	return nil
}

// GetSSOConfigPassword 获取解密后的 SSO 密码
func GetSSOConfigPassword() (string, error) {
	var encPwd string
	err := DB.QueryRow(`SELECT password FROM sso_config WHERE id = 1`).Scan(&encPwd)
	if err != nil {
		return "", err
	}
	return DecryptPassword(encPwd)
}

// --- Account CRUD ---

// CreateAccount 创建邮箱账户
func CreateAccount(acc *model.Account) error {
	encPwd, err := EncryptPassword(acc.Password)
	if err != nil {
		return fmt.Errorf("encrypt password: %w", err)
	}
	result, err := DB.Exec(`
		INSERT INTO accounts (email, display_name, imap_host, imap_port, encrypted_password, use_tls, status)
		VALUES (?, ?, ?, ?, ?, ?, ?)`,
		acc.Email, acc.DisplayName, acc.IMAPHost, acc.IMAPPort, encPwd, acc.UseTLS, acc.Status)
	if err != nil {
		return fmt.Errorf("insert account: %w", err)
	}
	id, _ := result.LastInsertId()
	acc.ID = id
	return nil
}

// ListAccounts 列出所有账户
func ListAccounts() ([]model.Account, error) {
	rows, err := DB.Query(`
		SELECT id, email, display_name, imap_host, imap_port, use_tls, status, last_error, last_sync_at, created_at, updated_at
		FROM accounts ORDER BY created_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var accounts []model.Account
	for rows.Next() {
		var acc model.Account
		var lastSync sql.NullTime
		if err := rows.Scan(&acc.ID, &acc.Email, &acc.DisplayName, &acc.IMAPHost, &acc.IMAPPort,
			&acc.UseTLS, &acc.Status, &acc.LastError, &lastSync, &acc.CreatedAt, &acc.UpdatedAt); err != nil {
			return nil, err
		}
		if lastSync.Valid {
			acc.LastSyncAt = &lastSync.Time
		}
		accounts = append(accounts, acc)
	}
	return accounts, rows.Err()
}

// GetAccount 获取单个账户
func GetAccount(id int64) (*model.Account, error) {
	var acc model.Account
	var lastSync sql.NullTime
	err := DB.QueryRow(`
		SELECT id, email, display_name, imap_host, imap_port, use_tls, status, last_error, last_sync_at, created_at, updated_at
		FROM accounts WHERE id = ?`, id,
	).Scan(&acc.ID, &acc.Email, &acc.DisplayName, &acc.IMAPHost, &acc.IMAPPort,
		&acc.UseTLS, &acc.Status, &acc.LastError, &lastSync, &acc.CreatedAt, &acc.UpdatedAt)
	if err != nil {
		return nil, err
	}
	if lastSync.Valid {
		acc.LastSyncAt = &lastSync.Time
	}
	return &acc, nil
}

// GetAccountPassword 获取解密后的密码
func GetAccountPassword(id int64) (string, error) {
	var encPwd string
	err := DB.QueryRow(`SELECT encrypted_password FROM accounts WHERE id = ?`, id).Scan(&encPwd)
	if err != nil {
		return "", err
	}
	return DecryptPassword(encPwd)
}

// UpdateAccount 更新账户
func UpdateAccount(acc *model.Account) error {
	if acc.Password != "" {
		encPwd, err := EncryptPassword(acc.Password)
		if err != nil {
			return fmt.Errorf("encrypt password: %w", err)
		}
		_, err = DB.Exec(`
			UPDATE accounts SET email=?, display_name=?, imap_host=?, imap_port=?, encrypted_password=?, use_tls=?, status=?, updated_at=?
			WHERE id=?`,
			acc.Email, acc.DisplayName, acc.IMAPHost, acc.IMAPPort, encPwd, acc.UseTLS, acc.Status, time.Now(), acc.ID)
		return err
	}
	_, err := DB.Exec(`
		UPDATE accounts SET email=?, display_name=?, imap_host=?, imap_port=?, use_tls=?, status=?, updated_at=?
		WHERE id=?`,
		acc.Email, acc.DisplayName, acc.IMAPHost, acc.IMAPPort, acc.UseTLS, acc.Status, time.Now(), acc.ID)
	return err
}

// DeleteAccount 删除账户
func DeleteAccount(id int64) error {
	_, err := DB.Exec(`DELETE FROM mails WHERE account_id=?`, id)
	if err != nil {
		return err
	}
	_, err = DB.Exec(`DELETE FROM accounts WHERE id=?`, id)
	return err
}

// UpdateAccountStatus 更新账户状态
func UpdateAccountStatus(id int64, status string, lastError string) error {
	_, err := DB.Exec(`UPDATE accounts SET status=?, last_error=?, updated_at=? WHERE id=?`,
		status, lastError, time.Now(), id)
	return err
}

// UpdateAccountSyncTime 更新同步时间
func UpdateAccountSyncTime(id int64) error {
	_, err := DB.Exec(`UPDATE accounts SET last_sync_at=?, status='active', last_error='', updated_at=? WHERE id=?`,
		time.Now(), time.Now(), id)
	return err
}

// --- Mail operations ---

// InsertMail 插入邮件（忽略重复）
func InsertMail(mail *model.MailItem) error {
	_, err := DB.Exec(`
		INSERT OR IGNORE INTO mails (account_id, message_id, from_addr, from_name, to_addr, subject, mail_date, size, is_read, has_attach, folder, body_text, body_html)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		mail.AccountID, mail.MessageID, mail.From, mail.FromName, mail.To, mail.Subject,
		mail.Date, mail.Size, mail.IsRead, mail.HasAttach, mail.Folder, mail.BodyText, mail.BodyHTML)
	return err
}

// BatchInsertMails 事务批量插入邮件
func BatchInsertMails(mails []model.MailItem) (int, error) {
	if len(mails) == 0 {
		return 0, nil
	}
	tx, err := DB.Begin()
	if err != nil {
		return 0, fmt.Errorf("begin transaction: %w", err)
	}
	defer tx.Rollback()

	stmt, err := tx.Prepare(`
		INSERT OR IGNORE INTO mails (account_id, message_id, from_addr, from_name, to_addr, subject, mail_date, size, is_read, has_attach, folder, body_text, body_html)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`)
	if err != nil {
		return 0, fmt.Errorf("prepare statement: %w", err)
	}
	defer stmt.Close()

	inserted := 0
	for _, m := range mails {
		result, err := stmt.Exec(m.AccountID, m.MessageID, m.From, m.FromName, m.To, m.Subject,
			m.Date, m.Size, m.IsRead, m.HasAttach, m.Folder, m.BodyText, m.BodyHTML)
		if err != nil {
			continue
		}
		rowsAffected, _ := result.RowsAffected()
		if rowsAffected > 0 {
			inserted++
		}
	}
	if err := tx.Commit(); err != nil {
		return 0, fmt.Errorf("commit transaction: %w", err)
	}
	return inserted, nil
}

// GetExistingMessageIDs 获取已有 message_id 集合
func GetExistingMessageIDs(accountID int64, folder string) (map[string]bool, error) {
	query := `SELECT message_id FROM mails WHERE account_id = ?`
	args := []interface{}{accountID}
	if folder != "" {
		query += ` AND folder = ?`
		args = append(args, folder)
	}
	rows, err := DB.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := make(map[string]bool)
	for rows.Next() {
		var msgID string
		if err := rows.Scan(&msgID); err != nil {
			continue
		}
		result[msgID] = true
	}
	return result, rows.Err()
}

// GetAccountMails 获取账户的邮件列表
func GetAccountMails(accountID int64, page, pageSize int, folder string) ([]model.MailItem, int64, error) {
	var total int64
	query := `SELECT COUNT(*) FROM mails WHERE account_id = ?`
	args := []interface{}{accountID}
	if folder != "" {
		query += ` AND folder = ?`
		args = append(args, folder)
	}
	if err := DB.QueryRow(query, args...).Scan(&total); err != nil {
		return nil, 0, err
	}

	offset := (page - 1) * pageSize
	query = `SELECT id, account_id, message_id, from_addr, from_name, to_addr, subject, mail_date, size, is_read, has_attach, folder, created_at
		FROM mails WHERE account_id = ?`
	args = []interface{}{accountID}
	if folder != "" {
		query += ` AND folder = ?`
		args = append(args, folder)
	}
	query += ` ORDER BY mail_date DESC LIMIT ? OFFSET ?`
	args = append(args, pageSize, offset)

	rows, err := DB.Query(query, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var mails []model.MailItem
	for rows.Next() {
		var m model.MailItem
		if err := rows.Scan(&m.ID, &m.AccountID, &m.MessageID, &m.From, &m.FromName,
			&m.To, &m.Subject, &m.Date, &m.Size, &m.IsRead, &m.HasAttach, &m.Folder, &m.CreatedAt); err != nil {
			return nil, 0, err
		}
		mails = append(mails, m)
	}
	return mails, total, rows.Err()
}

// GetOverview 获取总览统计
func GetOverview() (*model.OverviewStats, error) {
	stats := &model.OverviewStats{}
	DB.QueryRow(`SELECT COUNT(*) FROM accounts`).Scan(&stats.TotalAccounts)
	DB.QueryRow(`SELECT COUNT(*) FROM accounts WHERE status = 'active'`).Scan(&stats.ActiveAccounts)
	DB.QueryRow(`SELECT COUNT(*) FROM mails`).Scan(&stats.TotalMails)
	DB.QueryRow(`SELECT COUNT(*) FROM mails WHERE is_read = 0`).Scan(&stats.UnreadMails)
	DB.QueryRow(`SELECT COUNT(*) FROM mails WHERE date(mail_date) = date('now')`).Scan(&stats.TodayMails)
	DB.QueryRow(`SELECT COUNT(*) FROM mails WHERE mail_date >= datetime('now', '-7 days')`).Scan(&stats.WeekMails)
	return stats, nil
}

// GetMailSummaryPerAccount 按账户统计邮件
func GetMailSummaryPerAccount() ([]model.MailSummaryPerAccount, error) {
	rows, err := DB.Query(`
		SELECT a.id, a.email, a.display_name,
			COALESCE(COUNT(m.id), 0),
			COALESCE(SUM(CASE WHEN m.is_read = 0 THEN 1 ELSE 0 END), 0),
			COALESCE(SUM(CASE WHEN date(m.mail_date) = date('now') THEN 1 ELSE 0 END), 0),
			COALESCE(a.last_sync_at, '')
		FROM accounts a
		LEFT JOIN mails m ON a.id = m.account_id
		GROUP BY a.id
		ORDER BY a.created_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []model.MailSummaryPerAccount
	for rows.Next() {
		var r model.MailSummaryPerAccount
		if err := rows.Scan(&r.AccountID, &r.Email, &r.DisplayName,
			&r.TotalMails, &r.UnreadMails, &r.TodayMails, &r.LastSyncAt); err != nil {
			return nil, err
		}
		results = append(results, r)
	}
	return results, rows.Err()
}

// SearchMails 搜索邮件
func SearchMails(accountID int64, keyword string, page, pageSize int) ([]model.MailItem, int64, error) {
	var total int64
	pattern := "%" + keyword + "%"
	DB.QueryRow(`SELECT COUNT(*) FROM mails WHERE account_id = ? AND (subject LIKE ? OR from_addr LIKE ? OR from_name LIKE ?)`,
		accountID, pattern, pattern, pattern).Scan(&total)

	offset := (page - 1) * pageSize
	rows, err := DB.Query(`
		SELECT id, account_id, message_id, from_addr, from_name, to_addr, subject, mail_date, size, is_read, has_attach, folder, created_at
		FROM mails WHERE account_id = ? AND (subject LIKE ? OR from_addr LIKE ? OR from_name LIKE ?)
		ORDER BY mail_date DESC LIMIT ? OFFSET ?`,
		accountID, pattern, pattern, pattern, pageSize, offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var mails []model.MailItem
	for rows.Next() {
		var m model.MailItem
		if err := rows.Scan(&m.ID, &m.AccountID, &m.MessageID, &m.From, &m.FromName,
			&m.To, &m.Subject, &m.Date, &m.Size, &m.IsRead, &m.HasAttach, &m.Folder, &m.CreatedAt); err != nil {
			return nil, 0, err
		}
		mails = append(mails, m)
	}
	return mails, total, rows.Err()
}

// GetMailDetail 获取单封邮件详情
func GetMailDetail(id int64) (*model.MailItem, error) {
	var m model.MailItem
	err := DB.QueryRow(`
		SELECT id, account_id, message_id, from_addr, from_name, to_addr, subject, mail_date, size, is_read, has_attach, folder, body_text, body_html, created_at
		FROM mails WHERE id = ?`, id,
	).Scan(&m.ID, &m.AccountID, &m.MessageID, &m.From, &m.FromName,
		&m.To, &m.Subject, &m.Date, &m.Size, &m.IsRead, &m.HasAttach, &m.Folder,
		&m.BodyText, &m.BodyHTML, &m.CreatedAt)
	if err != nil {
		return nil, err
	}
	return &m, nil
}

// --- Patch operations ---

// GetUnparsedPatchMails 获取尚未解析的 Patch 邮件
func GetUnparsedPatchMails(accountID int64) ([]model.MailItem, error) {
	query := `
		SELECT id, account_id, message_id, from_addr, from_name, to_addr, subject, mail_date, size, is_read, has_attach, folder, created_at
		FROM mails
		WHERE subject LIKE '%Patch%' AND id NOT IN (SELECT mail_id FROM patch_infos)`
	args := []interface{}{}
	if accountID > 0 {
		query += ` AND account_id = ?`
		args = append(args, accountID)
	}
	rows, err := DB.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var mails []model.MailItem
	for rows.Next() {
		var m model.MailItem
		if err := rows.Scan(&m.ID, &m.AccountID, &m.MessageID, &m.From, &m.FromName,
			&m.To, &m.Subject, &m.Date, &m.Size, &m.IsRead, &m.HasAttach, &m.Folder, &m.CreatedAt); err != nil {
			return nil, err
		}
		mails = append(mails, m)
	}
	return mails, rows.Err()
}

// SavePatchInfos 批量保存 Patch 解析信息
func SavePatchInfos(infos []model.PatchInfo) error {
	if len(infos) == 0 {
		return nil
	}
	tx, err := DB.Begin()
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}
	defer tx.Rollback()

	stmt, err := tx.Prepare(`
		INSERT OR IGNORE INTO patch_infos (mail_id, account_id, subject, patch_type, product, version, patch_date, seq)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)`)
	if err != nil {
		return fmt.Errorf("prepare statement: %w", err)
	}
	defer stmt.Close()

	for _, info := range infos {
		_, err := stmt.Exec(info.MailID, info.AccountID, info.Subject, info.Type, info.Product, info.Version, info.Date, info.Seq)
		if err != nil {
			continue
		}
	}
	return tx.Commit()
}

// ParseAndSavePatchInfos 从给定邮件列表中解析 Patch 标题并保存
func ParseAndSavePatchInfos(mails []model.MailItem, parseFn func(string) *model.PatchInfo) int {
	var infos []model.PatchInfo
	for _, m := range mails {
		info := parseFn(m.Subject)
		if info == nil {
			continue
		}
		info.MailID = m.ID
		info.AccountID = m.AccountID
		infos = append(infos, *info)
	}
	if len(infos) > 0 {
		_ = SavePatchInfos(infos)
	}
	return len(infos)
}

// GetPatchSummaryByRange 从 patch_infos 表查询汇总
func GetPatchSummaryByRange(accountID int64, timeRange, startDate, endDate string) (*model.PatchSummaryResponse, error) {
	resp := &model.PatchSummaryResponse{
		Range:     timeRange,
		ByProduct: make(map[string]int),
		ByType:    make(map[string]int),
	}

	dateFilter := ""
	args := []interface{}{}

	if startDate != "" || endDate != "" {
		if startDate != "" {
			dateFilter += ` AND m.mail_date >= ?`
			args = append(args, startDate+" 00:00:00")
		}
		if endDate != "" {
			dateFilter += ` AND m.mail_date <= ?`
			args = append(args, endDate+" 23:59:59")
		}
		resp.Range = "custom"
	} else {
		switch timeRange {
		case "week":
			now := time.Now()
			weekday := int(now.Weekday())
			if weekday == 0 {
				weekday = 7
			}
			monday := time.Date(now.Year(), now.Month(), now.Day()-weekday+1, 0, 0, 0, 0, now.Location())
			dateFilter = ` AND m.mail_date >= ?`
			args = append(args, monday.Format("2006-01-02 15:04:05"))
		case "year":
			dateFilter = ` AND m.mail_date >= datetime('now', 'start of year')`
		}
	}

	if accountID > 0 {
		dateFilter += ` AND p.account_id = ?`
		args = append(args, accountID)
	}

	query := `
		SELECT p.mail_id, p.account_id, p.subject, p.patch_type, p.product, p.version, p.patch_date, p.seq,
		       m.from_name, m.from_addr, m.mail_date
		FROM patch_infos p
		JOIN mails m ON p.mail_id = m.id
		WHERE 1=1` + dateFilter + `
		ORDER BY m.mail_date DESC`

	rows, err := DB.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var info model.PatchInfo
		if err := rows.Scan(&info.MailID, &info.AccountID, &info.Subject, &info.Type,
			&info.Product, &info.Version, &info.Date, &info.Seq,
			&info.FromName, &info.FromAddr, &info.MailDate); err != nil {
			return nil, err
		}
		resp.Patches = append(resp.Patches, info)
		resp.ByProduct[info.Product]++
		resp.ByType[info.Type]++
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	resp.TotalCount = len(resp.Patches)
	return resp, nil
}

// GetPatchInfoByMailID 根据 mail_id 查询 patch_infos
func GetPatchInfoByMailID(mailID int64) (*model.PatchInfo, error) {
	var info model.PatchInfo
	err := DB.QueryRow(`
		SELECT p.mail_id, p.account_id, p.subject, p.patch_type, p.product, p.version, p.patch_date, p.seq,
		       m.from_name, m.from_addr, m.mail_date
		FROM patch_infos p
		JOIN mails m ON p.mail_id = m.id
		WHERE p.mail_id = ?`, mailID,
	).Scan(&info.MailID, &info.AccountID, &info.Subject, &info.Type,
		&info.Product, &info.Version, &info.Date, &info.Seq,
		&info.FromName, &info.FromAddr, &info.MailDate)
	if err != nil {
		return nil, err
	}
	return &info, nil
}

// --- AI Config operations ---

// SaveAIConfig 保存或更新 AI 配置
func SaveAIConfig(cfg *model.AIConfig) error {
	if cfg.ID > 0 {
		_, err := DB.Exec(`
			UPDATE ai_configs SET name=?, endpoint=?, api_key=?, model=?, is_default=?, updated_at=?
			WHERE id=?`,
			cfg.Name, cfg.Endpoint, cfg.APIKey, cfg.Model, cfg.IsDefault, time.Now(), cfg.ID)
		return err
	}
	result, err := DB.Exec(`
		INSERT INTO ai_configs (name, endpoint, api_key, model, is_default)
		VALUES (?, ?, ?, ?, ?)`,
		cfg.Name, cfg.Endpoint, cfg.APIKey, cfg.Model, cfg.IsDefault)
	if err != nil {
		return err
	}
	id, _ := result.LastInsertId()
	cfg.ID = id
	return nil
}

// ListAIConfigs 列出所有 AI 配置
func ListAIConfigs() ([]model.AIConfig, error) {
	rows, err := DB.Query(`
		SELECT id, name, endpoint, api_key, model, is_default, created_at, updated_at
		FROM ai_configs ORDER BY is_default DESC, created_at ASC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var configs []model.AIConfig
	for rows.Next() {
		var cfg model.AIConfig
		if err := rows.Scan(&cfg.ID, &cfg.Name, &cfg.Endpoint, &cfg.APIKey, &cfg.Model, &cfg.IsDefault, &cfg.CreatedAt, &cfg.UpdatedAt); err != nil {
			return nil, err
		}
		configs = append(configs, cfg)
	}
	return configs, rows.Err()
}

// GetDefaultAIConfig 获取默认 AI 配置
func GetDefaultAIConfig() (*model.AIConfig, error) {
	var cfg model.AIConfig
	err := DB.QueryRow(`
		SELECT id, name, endpoint, api_key, model, is_default, created_at, updated_at
		FROM ai_configs WHERE is_default = 1 LIMIT 1`,
	).Scan(&cfg.ID, &cfg.Name, &cfg.Endpoint, &cfg.APIKey, &cfg.Model, &cfg.IsDefault, &cfg.CreatedAt, &cfg.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return &cfg, nil
}

// DeleteAIConfig 删除 AI 配置
func DeleteAIConfig(id int64) error {
	_, err := DB.Exec(`DELETE FROM ai_configs WHERE id=?`, id)
	return err
}

// SetDefaultAIConfig 设置默认 AI 配置
func SetDefaultAIConfig(id int64) error {
	tx, err := DB.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if _, err := tx.Exec(`UPDATE ai_configs SET is_default=0`); err != nil {
		return err
	}
	if _, err := tx.Exec(`UPDATE ai_configs SET is_default=1 WHERE id=?`, id); err != nil {
		return err
	}
	return tx.Commit()
}

// --- Jira Config operations (保留向后兼容) ---

// GetJiraConfig 获取 JIRA 配置
func GetJiraConfig() (*model.JiraConfig, error) {
	var cfg model.JiraConfig
	err := DB.QueryRow(`
		SELECT id, username, password, base_url, created_at, updated_at
		FROM sso_config WHERE id = 1`,
	).Scan(&cfg.ID, &cfg.Username, &cfg.Password, &cfg.BaseURL, &cfg.CreatedAt, &cfg.UpdatedAt)
	if err != nil {
		return nil, err
	}
	if cfg.Password != "" {
		plain, decErr := DecryptPassword(cfg.Password)
		if decErr == nil {
			cfg.Password = plain
		}
	}
	return &cfg, nil
}

// GetJiraConfigPassword 获取解密后的 JIRA 密码
func GetJiraConfigPassword() (string, error) {
	return GetSSOConfigPassword()
}

// --- Setup Status helpers ---

// HasAccounts 检查是否存在邮箱账户
func HasAccounts() bool {
	var count int
	DB.QueryRow("SELECT COUNT(*) FROM accounts").Scan(&count)
	return count > 0
}

// HasJiraConfig 检查是否已配置 Jira
func HasJiraConfig() bool {
	var count int
	DB.QueryRow("SELECT COUNT(*) FROM sso_config WHERE username != ''").Scan(&count)
	return count > 0
}

// --- Settings 键值存储 ---

// GetSetting 获取设置值
func GetSetting(key string) (string, error) {
	var value string
	err := DB.QueryRow(`SELECT value FROM settings WHERE key = ?`, key).Scan(&value)
	if err == sql.ErrNoRows {
		return "", nil
	}
	return value, err
}

// SaveSetting 保存设置值
func SaveSetting(key, value string) error {
	_, err := DB.Exec(`INSERT INTO settings (key, value, updated_at) VALUES (?, ?, CURRENT_TIMESTAMP)
		ON CONFLICT(key) DO UPDATE SET value = excluded.value, updated_at = CURRENT_TIMESTAMP`, key, value)
	return err
}
