package service

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/lchaxian/patch-assistant/internal/db"
	"github.com/lchaxian/patch-assistant/internal/jira"
	"github.com/lchaxian/patch-assistant/internal/model"
	"github.com/lchaxian/patch-assistant/internal/wiki"
)

var jiraURLRegex = regexp.MustCompile(`https?://jira\.transwarp\.io/browse/([A-Z][A-Z0-9]+-\d+)`)
var warpKeyRegex = regexp.MustCompile(`(WARP-\d+)`)

type chatMessage struct {
	Role       string     `json:"role"`
	Content    string     `json:"content,omitempty"`
	ToolCalls  []toolCall `json:"tool_calls,omitempty"`
	ToolCallID string     `json:"tool_call_id,omitempty"`
}

type toolCall struct {
	ID       string       `json:"id"`
	Type     string       `json:"type"`
	Function toolFunction `json:"function"`
}

type toolFunction struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
}

type chatRequest struct {
	Model    string        `json:"model"`
	Messages []chatMessage `json:"messages"`
	Tools    []toolDef     `json:"tools,omitempty"`
}

type toolDef struct {
	Type     string      `json:"type"`
	Function toolFuncDef `json:"function"`
}

type toolFuncDef struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	Parameters  map[string]interface{} `json:"parameters"`
}

type chatResponse struct {
	Choices []struct {
		Message      chatMessage `json:"message"`
		FinishReason string      `json:"finish_reason"`
	} `json:"choices"`
}

var jiraToolDef = toolDef{
	Type: "function",
	Function: toolFuncDef{
		Name:        "query_warp_issue",
		Description: "根据 WARP 编号查询 JIRA Issue 详情（https://jira.transwarp.io）。返回 issue 的标题、描述、状态、分配人、报告人、创建时间、更新时间、标签、组件和评论列表。",
		Parameters: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"issue_key": map[string]interface{}{
					"type":        "string",
					"description": "WARP 编号，如 WARP-143944",
				},
			},
			"required": []string{"issue_key"},
		},
	},
}

var wikiToolDef = toolDef{
	Type: "function",
	Function: toolFuncDef{
		Name:        "search_wiki",
		Description: "搜索 Wiki（https://wiki.transwarp.io）上与指定 WARP 编号相关的文档和附件。返回标题中包含该 WARP 编号的相关结果及链接。对于页面类型结果，会自动获取页面正文和附件内容（如 SQL 文件），供深入分析。",
		Parameters: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"query": map[string]interface{}{
					"type":        "string",
					"description": "WARP 编号，如 WARP-138971",
				},
			},
			"required": []string{"query"},
		},
	},
}

var getWikiPageToolDef = toolDef{
	Type: "function",
	Function: toolFuncDef{
		Name:        "get_wiki_page",
		Description: "根据页面 ID 或 URL 获取 Wiki 页面的完整内容，包括正文和附件列表。对于文本类附件（如 SQL 文件），会自动下载并返回文件内容。适用于：1）search_wiki 搜索到的页面需要进一步查看完整内容时；2）已知 Wiki 页面 URL 或 ID，需要获取详情时。",
		Parameters: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"page_id": map[string]interface{}{
					"type":        "string",
					"description": "Wiki 页面 ID，如 138278371。也可传入完整 URL（如 https://wiki.transwarp.io/pages/viewpage.action?pageId=138278371），系统会自动提取 pageId",
				},
			},
			"required": []string{"page_id"},
		},
	},
}

// SummarizePatchMail 对指定 Patch 邮件进行 AI 汇总
func SummarizePatchMail(mailID int64, customPrompt string) (*model.AISummarizeResponse, error) {
	cfg, err := db.GetDefaultAIConfig()
	if err != nil {
		return nil, fmt.Errorf("请先配置 AI 服务：在 Patch 汇总页面点击齿轮图标添加 AI 配置")
	}

	mail, err := db.GetMailDetail(mailID)
	if err != nil {
		return nil, fmt.Errorf("邮件不存在: %w", err)
	}

	jiraLinks := extractJiraLinks(mail.BodyHTML, mail.BodyText)
	if len(jiraLinks) == 0 {
		warpKeys := extractWarpKeys(mail.BodyHTML, mail.BodyText)
		for _, key := range warpKeys {
			jiraLinks = append(jiraLinks, model.JiraLink{
				Key: key,
				URL: "https://jira.transwarp.io/browse/" + key,
			})
		}
	}

	mailContent := mail.BodyText
	if mailContent == "" && mail.BodyHTML != "" {
		mailContent = wiki.StripHTMLTags(mail.BodyHTML)
	}
	if mailContent == "" {
		return nil, fmt.Errorf("邮件没有正文内容，无法进行 AI 汇总")
	}

	prompt, _ := db.GetSetting("ai_prompt")
	if prompt == "" {
		prompt = db.DefaultPatchPrompt
	}
	if customPrompt != "" {
		prompt = customPrompt + "\n\n" + mailContent
	} else {
		prompt = prompt + mailContent
	}

	var tools []toolDef
	ssoCfg, ssoErr := db.GetSSOConfig()
	if ssoErr == nil {
		tools = append(tools, jiraToolDef)
		log.Printf("[AI] Jira 已配置，启用 query_warp_issue 工具")

		// Wiki 地址已配置时启用 search_wiki 和 get_wiki_page 工具
		wikiURL := ssoCfg.WikiURL
		if wikiURL == "" {
			wikiURL = "https://wiki.transwarp.io"
		}
		if wikiURL != "" {
			tools = append(tools, wikiToolDef, getWikiPageToolDef)
			log.Printf("[AI] Wiki 已配置（%s），启用 search_wiki 和 get_wiki_page 工具", wikiURL)
		}
	}

	summary, wikiResults, err := callAIWithTools(cfg, prompt, tools)
	if err != nil {
		return nil, fmt.Errorf("AI 调用失败: %w", err)
	}

	// 将 wiki 搜索结果转为 WikiLink
	var wikiLinks []model.WikiLink
	for _, r := range wikiResults {
		wikiLinks = append(wikiLinks, model.WikiLink{
			ID:    r.ID,
			Title: r.Title,
			URL:   r.URL,
		})
	}

	return &model.AISummarizeResponse{
		MailID:    mailID,
		Subject:   mail.Subject,
		Summary:   summary,
		Provider:  cfg.Name,
		Model:     cfg.Model,
		JiraLinks: jiraLinks,
		WikiLinks: wikiLinks,
	}, nil
}

func extractJiraLinks(bodyHTML, bodyText string) []model.JiraLink {
	seen := make(map[string]bool)
	var links []model.JiraLink

	for _, text := range []string{bodyHTML, bodyText} {
		if text == "" {
			continue
		}
		matches := jiraURLRegex.FindAllStringSubmatch(text, -1)
		for _, m := range matches {
			key := m[1]
			url := m[0]
			if !seen[key] {
				seen[key] = true
				links = append(links, model.JiraLink{Key: key, URL: url})
			}
		}
	}
	return links
}

func extractWarpKeys(bodyHTML, bodyText string) []string {
	seen := make(map[string]bool)
	var keys []string

	for _, text := range []string{bodyHTML, bodyText} {
		if text == "" {
			continue
		}
		matches := warpKeyRegex.FindAllString(text, -1)
		for _, m := range matches {
			if !seen[m] {
				seen[m] = true
				keys = append(keys, m)
			}
		}
	}
	return keys
}

func callAIWithTools(cfg *model.AIConfig, userPrompt string, tools []toolDef) (string, []wiki.SearchItem, error) {
	endpoint := strings.TrimRight(cfg.Endpoint, "/")
	url := endpoint
	if strings.HasSuffix(endpoint, "/chat/completions") {
		// already complete
	} else if strings.HasSuffix(endpoint, "/v1") {
		url = endpoint + "/chat/completions"
	} else {
		url = endpoint + "/v1/chat/completions"
	}

	messages := []chatMessage{
		{Role: "system", Content: "你是一个专业的软件 Patch 分析助手，请用中文回答。如果邮件中包含 WARP 开头的工单编号，请使用 query_warp_issue 工具查询工单详情，使用 search_wiki 工具搜索 Wiki 上的相关文档和附件（会自动获取页面正文和文本类附件内容），如果搜索结果内容不够详细，可使用 get_wiki_page 工具按页面 ID 获取完整内容。结合 JIRA 工单和 Wiki 文档内容进行更深入的分析，并基于这些内容生成测试案例。重要：JIRA 工单和 Wiki 文档内容必须直接内嵌展示在输出中，不要只放链接。用户应在当前页面就能看到完整信息，原文链接仅作为参考附在末尾。如果 Wiki 附件是 SQL、properties 等文本内容，用代码块包裹直接输出原文。"},
		{Role: "user", Content: userPrompt},
	}

	var wikiResults []wiki.SearchItem

	maxRounds := 5
	for round := 0; round < maxRounds; round++ {
		reqBody := chatRequest{
			Model:    cfg.Model,
			Messages: messages,
			Tools:    tools,
		}

		bodyBytes, err := json.Marshal(reqBody)
		if err != nil {
			return "", nil, fmt.Errorf("序列化请求失败: %w", err)
		}

		log.Printf("[AI] 请求 %s (round %d), 消息数: %d", cfg.Name, round+1, len(messages))

		client := &http.Client{Timeout: 120 * time.Second}
		req, err := http.NewRequest("POST", url, bytes.NewReader(bodyBytes))
		if err != nil {
			return "", nil, fmt.Errorf("创建请求失败: %w", err)
		}
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+cfg.APIKey)

		resp, err := client.Do(req)
		if err != nil {
			return "", nil, fmt.Errorf("请求失败: %w", err)
		}

		respBody, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil {
			return "", nil, fmt.Errorf("读取响应失败: %w", err)
		}

		if resp.StatusCode != http.StatusOK {
			return "", nil, fmt.Errorf("AI 返回错误 (%d): %s", resp.StatusCode, string(respBody))
		}

		var chatResp chatResponse
		if err := json.Unmarshal(respBody, &chatResp); err != nil {
			return "", nil, fmt.Errorf("解析响应失败: %w", err)
		}

		if len(chatResp.Choices) == 0 {
			return "", nil, fmt.Errorf("AI 未返回任何内容")
		}

		choice := chatResp.Choices[0]
		msg := choice.Message

		if len(msg.ToolCalls) == 0 {
			return msg.Content, wikiResults, nil
		}

		messages = append(messages, msg)

		for _, tc := range msg.ToolCalls {
			log.Printf("[AI] 工具调用: %s(%s)", tc.Function.Name, tc.Function.Arguments)

			var result string
			switch tc.Function.Name {
			case "query_warp_issue":
				result = executeQueryWarpIssue(tc.Function.Arguments)
			case "search_wiki":
				wikiResult, wikiItems := executeSearchWiki(tc.Function.Arguments)
				result = wikiResult
				wikiResults = append(wikiResults, wikiItems...)
			case "get_wiki_page":
				wikiResult, wikiItems := executeGetWikiPage(tc.Function.Arguments)
				result = wikiResult
				wikiResults = append(wikiResults, wikiItems...)
			default:
				result = fmt.Sprintf("未知工具: %s", tc.Function.Name)
			}

			messages = append(messages, chatMessage{
				Role:       "tool",
				Content:    result,
				ToolCallID: tc.ID,
			})
		}
	}

	return "", nil, fmt.Errorf("AI 工具调用超过最大轮数 (%d)", maxRounds)
}

func executeQueryWarpIssue(arguments string) string {
	var args struct {
		IssueKey string `json:"issue_key"`
	}
	if err := json.Unmarshal([]byte(arguments), &args); err != nil {
		return fmt.Sprintf(`{"error": "参数解析失败: %v"}`, err)
	}

	issueKey := strings.TrimSpace(args.IssueKey)
	if issueKey == "" {
		return `{"error": "issue_key 不能为空"}`
	}

	if !strings.Contains(issueKey, "-") {
		issueKey = "WARP-" + issueKey
	} else {
		issueKey = strings.ToUpper(issueKey)
	}

	log.Printf("[AI] 执行 query_warp_issue: issueKey=%s", issueKey)

	jiraCfg, err := db.GetSSOConfig()
	if err != nil {
		log.Printf("[AI] query_warp_issue 获取 SSO 配置失败: %v", err)
		return `{"error": "Jira 未配置，请先配置登录凭据"}`
	}

	password, err := db.GetSSOConfigPassword()
	if err != nil {
		log.Printf("[AI] query_warp_issue 获取密码失败: %v", err)
		return fmt.Sprintf(`{"error": "获取密码失败: %v"}`, err)
	}

	if jiraCfg.BaseURL == "" {
		jiraCfg.BaseURL = "https://jira.transwarp.io"
	}

	log.Printf("[AI] query_warp_issue 配置: baseURL=%s", jiraCfg.BaseURL)

	cfg := jira.Config{
		BaseURL:  jiraCfg.BaseURL,
		Username: jiraCfg.Username,
		Password: password,
	}

	issue, err := jira.GetIssue(cfg, issueKey)
	if err != nil {
		log.Printf("[AI] query_warp_issue 查询失败: %v", err)
		return fmt.Sprintf(`{"error": "查询 JIRA Issue %s 失败: %v"}`, issueKey, err)
	}

	result := map[string]interface{}{
		"key":       issue.Key,
		"url":       issue.URL,
		"summary":   issue.Fields.Summary,
		"status":    issue.Fields.Status.Name,
		"type":      issue.Fields.IssueType.Name,
		"priority":  issue.Fields.Priority.Name,
		"assignee":  issue.Fields.Assignee.DisplayName,
		"reporter":  issue.Fields.Reporter.DisplayName,
		"created":   issue.Fields.Created,
		"updated":   issue.Fields.Updated,
		"labels":    issue.Fields.Labels,
	}

	if len(issue.Fields.Components) > 0 {
		var comps []string
		for _, c := range issue.Fields.Components {
			comps = append(comps, c.Name)
		}
		result["components"] = comps
	}

	if issue.Fields.Description != "" {
		desc := strings.TrimSpace(issue.Fields.Description)
		if len(desc) > 2000 {
			desc = desc[:2000] + "...(截断)"
		}
		result["description"] = desc
	}

	if issue.Fields.Comments != nil && len(issue.Fields.Comments.Values) > 0 {
		var comments []map[string]interface{}
		for i, c := range issue.Fields.Comments.Values {
			if i >= 5 {
				comments = append(comments, map[string]interface{}{
					"note":  fmt.Sprintf("还有 %d 条评论未显示", len(issue.Fields.Comments.Values)-i),
					"total": len(issue.Fields.Comments.Values),
				})
				break
			}
			body := strings.TrimSpace(c.Body)
			if len(body) > 500 {
				body = body[:500] + "...(截断)"
			}
			comments = append(comments, map[string]interface{}{
				"author":  c.Author.DisplayName,
				"created": c.Created,
				"body":    body,
			})
		}
		result["comments"] = comments
		result["comment_count"] = len(issue.Fields.Comments.Values)
	}

	jsonBytes, err := json.Marshal(result)
	if err != nil {
		return fmt.Sprintf(`{"error": "序列化结果失败: %v"}`, err)
	}

	log.Printf("[AI] 查询 JIRA Issue %s 成功: %s", issue.Key, issue.Fields.Summary)
	return string(jsonBytes)
}

func executeSearchWiki(arguments string) (string, []wiki.SearchItem) {
	var args struct {
		Query string `json:"query"`
	}
	if err := json.Unmarshal([]byte(arguments), &args); err != nil {
		return fmt.Sprintf(`{"error": "参数解析失败: %v"}`, err), nil
	}

	query := strings.TrimSpace(args.Query)
	if query == "" {
		return `{"error": "query 不能为空"}`, nil
	}

	log.Printf("[AI] 执行 search_wiki: query=%s", query)

	ssoCfg, err := db.GetSSOConfig()
	if err != nil {
		log.Printf("[AI] search_wiki 获取 SSO 配置失败: %v", err)
		return `{"error": "Wiki 未配置，请先配置登录凭据"}`, nil
	}

	password, err := db.GetSSOConfigPassword()
	if err != nil {
		log.Printf("[AI] search_wiki 获取密码失败: %v", err)
		return fmt.Sprintf(`{"error": "获取密码失败: %v"}`, err), nil
	}

	wikiURL := ssoCfg.WikiURL
	if wikiURL == "" {
		wikiURL = "https://wiki.transwarp.io"
	}

	log.Printf("[AI] search_wiki 配置: wikiURL=%s", wikiURL)

	cfg := wiki.Config{
		BaseURL:  wikiURL,
		Username: ssoCfg.Username,
		Password: password,
	}

	searchResult, err := wiki.SearchWiki(cfg, query)
	if err != nil {
		log.Printf("[AI] search_wiki 搜索失败: %v", err)
		return fmt.Sprintf(`{"error": "搜索 Wiki 失败: %v"}`, err), nil
	}

	if len(searchResult.Results) == 0 {
		return `{"message": "未找到相关 Wiki 文档", "results": []}`, nil
	}

	// 构建返回给 AI 的结构化数据
	var results []map[string]interface{}
	for _, item := range searchResult.Results {
		r := map[string]interface{}{
			"id":    item.ID,
			"title": item.Title,
			"type":  item.Type,
			"url":   item.URL,
		}
		// 返回页面正文或附件内容
		if item.Content != "" {
			content := item.Content
			if len(content) > 5000 {
				content = content[:5000] + "\n...(截断)"
			}
			r["content"] = content
		}
		// 搜索摘要
		if item.Excerpt != "" {
			r["excerpt"] = item.Excerpt
		}
		results = append(results, r)
	}

	jsonBytes, err := json.Marshal(map[string]interface{}{
		"total":   len(results),
		"results": results,
	})
	if err != nil {
		return fmt.Sprintf(`{"error": "序列化结果失败: %v"}`, err), nil
	}

	log.Printf("[AI] 搜索 Wiki '%s' 成功，找到 %d 条相关结果", query, len(results))
	return string(jsonBytes), searchResult.Results
}

// executeGetWikiPage 处理 AI 的 get_wiki_page 工具调用
// 根据 page_id 或 URL 获取 Wiki 页面详情（正文+附件内容）
func executeGetWikiPage(arguments string) (string, []wiki.SearchItem) {
	var args struct {
		PageID string `json:"page_id"`
	}
	if err := json.Unmarshal([]byte(arguments), &args); err != nil {
		return fmt.Sprintf(`{"error": "参数解析失败: %v"}`, err), nil
	}

	pageID := strings.TrimSpace(args.PageID)
	if pageID == "" {
		return `{"error": "page_id 不能为空"}`, nil
	}

	log.Printf("[AI] 执行 get_wiki_page: page_id=%s", pageID)

	ssoCfg, err := db.GetSSOConfig()
	if err != nil {
		log.Printf("[AI] get_wiki_page 获取 SSO 配置失败: %v", err)
		return `{"error": "Wiki 未配置，请先配置登录凭据"}`, nil
	}

	password, err := db.GetSSOConfigPassword()
	if err != nil {
		log.Printf("[AI] get_wiki_page 获取密码失败: %v", err)
		return fmt.Sprintf(`{"error": "获取密码失败: %v"}`, err), nil
	}

	wikiURL := ssoCfg.WikiURL
	if wikiURL == "" {
		wikiURL = "https://wiki.transwarp.io"
	}

	cfg := wiki.Config{
		BaseURL:  wikiURL,
		Username: ssoCfg.Username,
		Password: password,
	}

	// 判断是纯 ID 还是完整 URL
	var pageDetail *wiki.PageDetail
	if isNumeric(pageID) {
		pageDetail, err = wiki.GetPageContent(cfg, pageID)
	} else {
		// 尝试作为 URL 解析
		pageDetail, err = wiki.FetchWikiPageByURL(cfg, pageID)
	}

	if err != nil {
		log.Printf("[AI] get_wiki_page 获取页面失败: %v", err)
		return fmt.Sprintf(`{"error": "获取 Wiki 页面失败: %v"}`, err), nil
	}

	// 构建返回给 AI 的结构化数据
	r := map[string]interface{}{
		"id":    pageDetail.ID,
		"title": pageDetail.Title,
		"type":  pageDetail.Type,
		"url":   pageDetail.URL,
	}

	// 页面正文（去除 HTML 标签）
	if pageDetail.Body != "" {
		body := wiki.StripHTMLTags(pageDetail.Body)
		if len(body) > 8000 {
			body = body[:8000] + "\n...(截断)"
		}
		r["body"] = body
	}

	// 附件信息
	if len(pageDetail.Attachments) > 0 {
		var attList []map[string]interface{}
		for _, att := range pageDetail.Attachments {
			attInfo := map[string]interface{}{
				"id":       att.ID,
				"title":    att.Title,
				"size":     att.FileSize,
				"type":     att.MediaType,
				"url":      att.DownloadURL,
			}
			if att.Content != "" {
				attInfo["content"] = att.Content
			}
			attList = append(attList, attInfo)
		}
		r["attachments"] = attList
	}

	jsonBytes, err := json.Marshal(r)
	if err != nil {
		return fmt.Sprintf(`{"error": "序列化结果失败: %v"}`, err), nil
	}

	log.Printf("[AI] get_wiki_page 成功，页面: %s (ID: %s)", pageDetail.Title, pageDetail.ID)

	// 将页面详情转为 SearchItem 用于链接展示
	wikiItems := []wiki.SearchItem{
		{
			ID:      pageDetail.ID,
			Title:   pageDetail.Title,
			Type:    pageDetail.Type,
			URL:     pageDetail.URL,
			Content: pageDetail.Body,
		},
	}

	return string(jsonBytes), wikiItems
}

// isNumeric 判断字符串是否为纯数字
func isNumeric(s string) bool {
	for _, c := range s {
		if c < '0' || c > '9' {
			return false
		}
	}
	return len(s) > 0
}
