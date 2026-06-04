package handler

import (
	"encoding/json"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/lchaxian/patch-assistant/internal/db"
	"github.com/lchaxian/patch-assistant/internal/jira"
	"github.com/lchaxian/patch-assistant/internal/model"
	"github.com/lchaxian/patch-assistant/internal/service"
)

// --- Account Handlers ---

func ListAccounts(c *gin.Context) {
	accounts, err := db.ListAccounts()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if accounts == nil {
		accounts = []model.Account{}
	}
	c.JSON(http.StatusOK, gin.H{"data": accounts})
}

func CreateAccount(c *gin.Context) {
	var req struct {
		Email       string `json:"email" binding:"required,email"`
		DisplayName string `json:"display_name"`
		Password    string `json:"password" binding:"required,min=1"`
		IMAPHost    string `json:"imap_host"`
		IMAPPort    int    `json:"imap_port"`
		UseTLS      *bool  `json:"use_tls"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "参数错误: " + err.Error()})
		return
	}
	acc := &model.Account{
		Email:       req.Email,
		DisplayName: req.DisplayName,
		Password:    req.Password,
		IMAPHost:    req.IMAPHost,
		IMAPPort:    req.IMAPPort,
		UseTLS:      true,
		Status:      "active",
	}
	if acc.IMAPHost == "" {
		acc.IMAPHost = "imap.exmail.qq.com"
	}
	if acc.IMAPPort == 0 {
		acc.IMAPPort = 993
	}
	if req.UseTLS != nil {
		acc.UseTLS = *req.UseTLS
	}
	if err := db.CreateAccount(acc); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "创建失败: " + err.Error()})
		return
	}
	c.JSON(http.StatusCreated, gin.H{"data": acc})
}

func GetAccount(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的 ID"})
		return
	}
	acc, err := db.GetAccount(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "账户不存在"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": acc})
}

func UpdateAccount(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的 ID"})
		return
	}
	var req struct {
		Email       string `json:"email" binding:"required,email"`
		DisplayName string `json:"display_name"`
		Password    string `json:"password"`
		IMAPHost    string `json:"imap_host"`
		IMAPPort    int    `json:"imap_port"`
		UseTLS      *bool  `json:"use_tls"`
		Status      string `json:"status"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "参数错误: " + err.Error()})
		return
	}
	acc := &model.Account{
		ID:          id,
		Email:       req.Email,
		DisplayName: req.DisplayName,
		Password:    req.Password,
		IMAPHost:    req.IMAPHost,
		IMAPPort:    req.IMAPPort,
		UseTLS:      true,
		Status:      req.Status,
	}
	if acc.IMAPHost == "" {
		acc.IMAPHost = "imap.exmail.qq.com"
	}
	if acc.IMAPPort == 0 {
		acc.IMAPPort = 993
	}
	if req.UseTLS != nil {
		acc.UseTLS = *req.UseTLS
	}
	if acc.Status == "" {
		acc.Status = "active"
	}
	if err := db.UpdateAccount(acc); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "更新失败: " + err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": acc})
}

func DeleteAccount(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的 ID"})
		return
	}
	if err := db.DeleteAccount(id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "删除失败: " + err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "删除成功"})
}

func TestAccountConnection(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的 ID"})
		return
	}
	acc, err := db.GetAccount(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "账户不存在"})
		return
	}
	password, err := db.GetAccountPassword(id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "获取密码失败"})
		return
	}
	config := service.IMAPConfig{
		Host:     acc.IMAPHost,
		Port:     acc.IMAPPort,
		Email:    acc.Email,
		Password: password,
		UseTLS:   acc.UseTLS,
	}
	if err := service.TestConnection(config); err != nil {
		_ = db.UpdateAccountStatus(id, "error", err.Error())
		c.JSON(http.StatusOK, gin.H{"success": false, "error": err.Error()})
		return
	}
	_ = db.UpdateAccountStatus(id, "active", "")
	c.JSON(http.StatusOK, gin.H{"success": true, "message": "连接成功"})
}

// --- Mail Handlers ---

func GetMailSummary(c *gin.Context) {
	summaries, err := db.GetMailSummaryPerAccount()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if summaries == nil {
		summaries = []model.MailSummaryPerAccount{}
	}
	c.JSON(http.StatusOK, gin.H{"data": summaries})
}

func GetAccountMails(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的 ID"})
		return
	}
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "50"))
	folder := c.Query("folder")
	keyword := c.Query("keyword")
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 200 {
		pageSize = 50
	}

	var mails []model.MailItem
	var total int64

	if keyword != "" {
		mails, total, err = db.SearchMails(id, keyword, page, pageSize)
	} else {
		mails, total, err = db.GetAccountMails(id, page, pageSize, folder)
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if mails == nil {
		mails = []model.MailItem{}
	}
	c.JSON(http.StatusOK, gin.H{
		"data": mails,
		"pagination": gin.H{
			"page":       page,
			"page_size":  pageSize,
			"total":      total,
			"total_page": (total + int64(pageSize) - 1) / int64(pageSize),
		},
	})
}

func SyncAccountMails(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的 ID"})
		return
	}
	days, _ := strconv.Atoi(c.DefaultQuery("days", "30"))
	result, err := service.SyncMails(id, days)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "同步失败: " + err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": result})
}

func GetMailDetail(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的 ID"})
		return
	}
	mail, err := db.GetMailDetail(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "邮件不存在"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": mail})
}

// --- Stats Handlers ---

func GetOverview(c *gin.Context) {
	stats, err := db.GetOverview()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": stats})
}

// --- Patch Summary Handler ---

func GetPatchSummary(c *gin.Context) {
	timeRange := c.DefaultQuery("range", "week")
	allowedRanges := map[string]bool{"7d": true, "30d": true, "90d": true, "week": true, "year": true, "custom": true}
	if !allowedRanges[timeRange] {
		timeRange = "week"
	}
	startDate := c.Query("start_date")
	endDate := c.Query("end_date")
	needSync := c.Query("sync") == "true"
	accountIDStr := c.Query("account_id")
	var accountID int64
	if accountIDStr != "" {
		if id, err := strconv.ParseInt(accountIDStr, 10, 64); err == nil {
			accountID = id
		}
	}

	var syncResults []model.SyncResult
	if needSync {
		var sinceDate time.Time
		now := time.Now()
		if startDate != "" {
			parsed, err := time.Parse("2006-01-02", startDate)
			if err == nil {
				sinceDate = parsed
			} else {
				sinceDate = now.AddDate(0, 0, -7)
			}
		} else {
			switch timeRange {
			case "year":
				sinceDate = time.Date(now.Year(), 1, 1, 0, 0, 0, 0, now.Location())
			case "7d":
				sinceDate = now.AddDate(0, 0, -7)
			case "30d":
				sinceDate = now.AddDate(0, 0, -30)
			case "90d":
				sinceDate = now.AddDate(0, 0, -90)
			default: // week
				weekday := int(now.Weekday())
				if weekday == 0 {
					weekday = 7
				}
				sinceDate = time.Date(now.Year(), now.Month(), now.Day()-weekday+1, 0, 0, 0, 0, now.Location())
			}
		}

		if accountID > 0 {
			acc, _ := db.GetAccount(accountID)
			result, err := service.SyncMailsSince(accountID, sinceDate)
			if err == nil && result != nil {
				if acc != nil {
					result.AccountEmail = acc.Email
				}
				syncResults = append(syncResults, *result)
			}
		} else {
			accounts, err := db.ListAccounts()
			if err == nil {
				for _, acc := range accounts {
					result, err := service.SyncMailsSince(acc.ID, sinceDate)
					if err == nil && result != nil {
						result.AccountEmail = acc.Email
						syncResults = append(syncResults, *result)
					}
				}
			}
		}
	}

	// 填充同步结果中的 Patch 维度数据
	if len(syncResults) > 0 {
		for i := range syncResults {
			// 每个账户单独计算新解析的 Patch 数
			syncResults[i].NewPatchMails = service.ParseAndSaveNewPatchMails(syncResults[i].AccountID)
		}
	}

	resp, err := db.GetPatchSummaryByRange(accountID, timeRange, startDate, endDate)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "查询失败: " + err.Error()})
		return
	}

	if len(syncResults) > 0 {
		for i := range syncResults {
			syncResults[i].RangePatchTotal = resp.TotalCount
		}
		resp.SyncResults = syncResults
	}
	c.JSON(http.StatusOK, gin.H{"data": resp})
}

// GetPatchSyncStatus 返回各账户最近同步时间，用于前端判断是否需要提醒用户刷新
func GetPatchSyncStatus(c *gin.Context) {
	accounts, err := db.ListAccounts()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	type accountSyncStatus struct {
		AccountID  int64  `json:"account_id"`
		Email      string `json:"email"`
		LastSyncAt string `json:"last_sync_at"`
	}
	var statuses []accountSyncStatus
	for _, acc := range accounts {
		s := accountSyncStatus{
			AccountID: acc.ID,
			Email:     acc.Email,
		}
		if acc.LastSyncAt != nil {
			s.LastSyncAt = acc.LastSyncAt.Format("2006-01-02 15:04:05")
		}
		statuses = append(statuses, s)
	}
	c.JSON(http.StatusOK, gin.H{"data": statuses})
}

// GetPatchSources 获取 Patch 来源邮箱列表
func GetPatchSources(c *gin.Context) {
	val, err := db.GetSetting("patch_source_emails")
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if val == "" {
		c.JSON(http.StatusOK, gin.H{"data": []string{}})
		return
	}
	var emails []string
	if err := json.Unmarshal([]byte(val), &emails); err != nil {
		c.JSON(http.StatusOK, gin.H{"data": []string{}})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": emails})
}

// SavePatchSources 保存 Patch 来源邮箱列表
func SavePatchSources(c *gin.Context) {
	var req struct {
		Emails []string `json:"emails" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "参数错误"})
		return
	}
	// 去重去空
	seen := map[string]bool{}
	cleaned := []string{}
	for _, e := range req.Emails {
		e = strings.TrimSpace(e)
		if e != "" && !seen[e] {
			seen[e] = true
			cleaned = append(cleaned, e)
		}
	}
	data, _ := json.Marshal(cleaned)
	if err := db.SaveSetting("patch_source_emails", string(data)); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "保存失败"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": cleaned})
}

// GetPatchIMAPCount 轻量计数：只做 IMAP SEARCH，快速返回服务器上 Patch 邮件总数和已同步数
func GetPatchIMAPCount(c *gin.Context) {
	timeRange := c.DefaultQuery("range", "30d")
	accountIDStr := c.Query("account_id")
	var accountID int64
	if accountIDStr != "" {
		if id, err := strconv.ParseInt(accountIDStr, 10, 64); err == nil {
			accountID = id
		}
	}

	sinceDate := calcSinceDate(timeRange)

	var results []service.PatchCountResult
	if accountID > 0 {
		r, err := service.CountPatchOnServer(accountID, sinceDate)
		if err != nil {
			c.JSON(http.StatusOK, gin.H{"data": []service.PatchCountResult{{AccountID: accountID, Error: err.Error()}}})
			return
		}
		results = append(results, *r)
	} else {
		accounts, err := db.ListAccounts()
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		for _, acc := range accounts {
			r, err := service.CountPatchOnServer(acc.ID, sinceDate)
			if err != nil {
				results = append(results, service.PatchCountResult{AccountID: acc.ID, AccountEmail: acc.Email, Error: err.Error()})
				continue
			}
			results = append(results, *r)
		}
	}
	c.JSON(http.StatusOK, gin.H{"data": results})
}

// calcSinceDate 根据时间范围计算起始日期
func calcSinceDate(timeRange string) time.Time {
	now := time.Now()
	switch timeRange {
	case "year":
		return time.Date(now.Year(), 1, 1, 0, 0, 0, 0, now.Location())
	case "7d":
		return now.AddDate(0, 0, -7)
	case "30d":
		return now.AddDate(0, 0, -30)
	case "90d":
		return now.AddDate(0, 0, -90)
	default: // week
		weekday := int(now.Weekday())
		if weekday == 0 {
			weekday = 7
		}
		return time.Date(now.Year(), now.Month(), now.Day()-weekday+1, 0, 0, 0, 0, now.Location())
	}
}

// SearchPatch 搜索本地 Patch 数据库
func SearchPatch(c *gin.Context) {
	var req struct {
		Keyword   string `json:"keyword"`
		AccountID int64  `json:"account_id"`
	}
	if err := c.BindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的请求参数"})
		return
	}

	keyword := strings.TrimSpace(req.Keyword)
	if keyword == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "搜索关键词不能为空"})
		return
	}

	patches, err := db.SearchPatches(req.AccountID, keyword, 50)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "搜索失败: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": model.PatchSearchResponse{
		Patches: patches,
	}})
}

// --- AI Config Handlers ---

func ListAIConfigs(c *gin.Context) {
	configs, err := db.ListAIConfigs()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if configs == nil {
		configs = []model.AIConfig{}
	}
	c.JSON(http.StatusOK, gin.H{"data": configs})
}

func CreateAIConfig(c *gin.Context) {
	var req struct {
		Name      string `json:"name" binding:"required"`
		Endpoint  string `json:"endpoint" binding:"required"`
		APIKey    string `json:"api_key" binding:"required"`
		Model     string `json:"model" binding:"required"`
		IsDefault bool   `json:"is_default"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "参数错误: " + err.Error()})
		return
	}
	cfg := &model.AIConfig{
		Name:      req.Name,
		Endpoint:  req.Endpoint,
		APIKey:    req.APIKey,
		Model:     req.Model,
		IsDefault: req.IsDefault,
	}
	if err := db.SaveAIConfig(cfg); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "创建失败: " + err.Error()})
		return
	}
	if cfg.IsDefault {
		_ = db.SetDefaultAIConfig(cfg.ID)
	}
	c.JSON(http.StatusCreated, gin.H{"data": cfg})
}

func UpdateAIConfig(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的 ID"})
		return
	}
	var req struct {
		Name      string `json:"name" binding:"required"`
		Endpoint  string `json:"endpoint" binding:"required"`
		APIKey    string `json:"api_key" binding:"required"`
		Model     string `json:"model" binding:"required"`
		IsDefault bool   `json:"is_default"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "参数错误: " + err.Error()})
		return
	}
	cfg := &model.AIConfig{
		ID:        id,
		Name:      req.Name,
		Endpoint:  req.Endpoint,
		APIKey:    req.APIKey,
		Model:     req.Model,
		IsDefault: req.IsDefault,
	}
	if err := db.SaveAIConfig(cfg); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "更新失败: " + err.Error()})
		return
	}
	if cfg.IsDefault {
		_ = db.SetDefaultAIConfig(cfg.ID)
	}
	c.JSON(http.StatusOK, gin.H{"data": cfg})
}

func DeleteAIConfig(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的 ID"})
		return
	}
	if err := db.DeleteAIConfig(id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "删除失败: " + err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "删除成功"})
}

func SetDefaultAIConfigHandler(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的 ID"})
		return
	}
	if err := db.SetDefaultAIConfig(id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "设置失败: " + err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "设置成功"})
}

// --- Setup Status Handler ---

func GetSetupStatus(c *gin.Context) {
	setupCompleted, _ := db.GetSetting("setup_completed")
	c.JSON(http.StatusOK, gin.H{
		"data": gin.H{
			"has_accounts":    db.HasAccounts(),
			"has_jira_config": db.HasJiraConfig(),
			"setup_completed": setupCompleted == "true",
		},
	})
}

func CompleteSetup(c *gin.Context) {
	if err := db.SaveSetting("setup_completed", "true"); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": gin.H{"setup_completed": true}})
}

// --- JIRA Config Handlers ---

func GetJiraConfig(c *gin.Context) {
	cfg, err := db.GetSSOConfig()
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"data": model.SSOConfig{}})
		return
	}
	cfg.Password = ""
	c.JSON(http.StatusOK, gin.H{"data": cfg})
}

func SaveJiraConfig(c *gin.Context) {
	var req struct {
		Username string `json:"username" binding:"required"`
		Password string `json:"password" binding:"required"`
		BaseURL  string `json:"base_url"`
		LoginURL string `json:"login_url"`
		WikiURL  string `json:"wiki_url"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "参数错误: " + err.Error()})
		return
	}
	baseURL := req.BaseURL
	if baseURL == "" {
		baseURL = "https://jira.transwarp.io"
	}
	wikiURL := req.WikiURL
	if wikiURL == "" {
		wikiURL = "https://wiki.transwarp.io"
	}

	// 验证 JIRA 凭据
	if err := jira.TestAuth(jira.Config{
		BaseURL:  baseURL,
		Username: req.Username,
		Password: req.Password,
	}); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "JIRA 验证失败: " + err.Error()})
		return
	}

	cfg := &model.SSOConfig{
		Username: req.Username,
		Password: req.Password,
		BaseURL:  baseURL,
		LoginURL: req.LoginURL,
		WikiURL:  wikiURL,
	}
	if err := db.SaveSSOConfig(cfg); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "保存失败: " + err.Error()})
		return
	}
	cfg.Password = ""
	c.JSON(http.StatusOK, gin.H{"data": cfg})
}

// --- AI Summarize Handler ---

func GetAISummary(c *gin.Context) {
	mailID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的 ID"})
		return
	}
	summary, err := db.GetAISummary(mailID)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"data": nil})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": summary})
}

func BatchGetAISummaries(c *gin.Context) {
	var req struct {
		MailIDs []int64 `json:"mail_ids"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "参数错误: " + err.Error()})
		return
	}
	summaries, err := db.GetAISummariesByMailIDs(req.MailIDs)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	// 转为 map[mail_id]AISummarizeResponse 格式
	result := make(map[int64]*model.AISummarizeResponse)
	for mailID, s := range summaries {
		result[mailID] = &model.AISummarizeResponse{
			MailID:    s.MailID,
			Subject:   s.Subject,
			Summary:   s.Summary,
			Provider:  s.Provider,
			Model:     s.Model,
			JiraLinks: s.JiraLinks,
			WikiLinks: s.WikiLinks,
			CreatedAt: s.CreatedAt.Format("2006-01-02 15:04:05"),
		}
	}
	c.JSON(http.StatusOK, gin.H{"data": result})
}

func AISummarize(c *gin.Context) {
	var req model.AISummarizeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "参数错误: " + err.Error()})
		return
	}

	// 如果不是强制刷新，先尝试从缓存读取
	if !req.Force {
		cached, err := db.GetAISummary(req.MailID)
		if err == nil && cached != nil {
			// 将 AISummary 转为 AISummarizeResponse 格式返回
			result := &model.AISummarizeResponse{
				MailID:    cached.MailID,
				Subject:   cached.Subject,
				Summary:   cached.Summary,
				Provider:  cached.Provider,
				Model:     cached.Model,
				JiraLinks: cached.JiraLinks,
				WikiLinks: cached.WikiLinks,
				CreatedAt: cached.CreatedAt.Format("2006-01-02 15:04:05"),
			}
			c.JSON(http.StatusOK, gin.H{"data": result, "cached": true})
			return
		}
	}

	// 执行 AI 汇总
	result, err := service.SummarizePatchMail(req.MailID, req.Prompt)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// 持久化汇总结果
	summary := &model.AISummary{
		MailID:    result.MailID,
		Subject:   result.Subject,
		Summary:   result.Summary,
		Provider:  result.Provider,
		Model:     result.Model,
		JiraLinks: result.JiraLinks,
		WikiLinks: result.WikiLinks,
	}
	if saveErr := db.SaveAISummary(summary); saveErr != nil {
		log.Printf("[WARN] 保存 AI 汇总结果失败: %v", saveErr)
	} else {
		// 回填 created_at 给响应
		result.CreatedAt = summary.CreatedAt.Format("2006-01-02 15:04:05")
	}

	c.JSON(http.StatusOK, gin.H{"data": result, "cached": false})
}

// --- AI Prompt Handlers ---

func GetAIPrompt(c *gin.Context) {
	prompt, err := db.GetSetting("ai_prompt")
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if prompt == "" {
		prompt = db.DefaultPatchPrompt
	}
	c.JSON(http.StatusOK, gin.H{"data": gin.H{"prompt": prompt}})
}

func SaveAIPrompt(c *gin.Context) {
	var req struct {
		Prompt string `json:"prompt"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "参数错误: " + err.Error()})
		return
	}
	if err := db.SaveSetting("ai_prompt", req.Prompt); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "保存失败: " + err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": gin.H{"prompt": req.Prompt}})
}
