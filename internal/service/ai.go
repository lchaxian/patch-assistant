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
)

var jiraURLRegex = regexp.MustCompile(`https?://jira\.transwarp\.io/browse/([A-Z][A-Z0-9]+-\d+)`)
var warpKeyRegex = regexp.MustCompile(`(WARP-\d+)`)

const defaultPatchPrompt = `你是一个专业的软件 Patch 分析助手。请根据以下 Patch 发布通知邮件内容，生成一份结构化的 Patch 调整摘要。

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
`

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
		mailContent = stripHTMLTags(mail.BodyHTML)
	}
	if mailContent == "" {
		return nil, fmt.Errorf("邮件没有正文内容，无法进行 AI 汇总")
	}

	prompt, _ := db.GetSetting("ai_prompt")
	if prompt == "" {
		prompt = defaultPatchPrompt
	}
	if customPrompt != "" {
		prompt = customPrompt + "\n\n" + mailContent
	} else {
		prompt = prompt + mailContent
	}

	var tools []toolDef
	if _, err := db.GetSSOConfig(); err == nil {
		tools = []toolDef{jiraToolDef}
		log.Printf("[AI] Jira 已配置，启用 query_warp_issue 工具")
	}

	summary, err := callAIWithTools(cfg, prompt, tools)
	if err != nil {
		return nil, fmt.Errorf("AI 调用失败: %w", err)
	}

	return &model.AISummarizeResponse{
		MailID:    mailID,
		Subject:   mail.Subject,
		Summary:   summary,
		Provider:  cfg.Name,
		Model:     cfg.Model,
		JiraLinks: jiraLinks,
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

func callAIWithTools(cfg *model.AIConfig, userPrompt string, tools []toolDef) (string, error) {
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
		{Role: "system", Content: "你是一个专业的软件 Patch 分析助手，请用中文回答。如果邮件中包含 WARP 开头的工单编号，请使用 query_warp_issue 工具查询工单详情，结合查询结果进行更深入的分析。"},
		{Role: "user", Content: userPrompt},
	}

	maxRounds := 5
	for round := 0; round < maxRounds; round++ {
		reqBody := chatRequest{
			Model:    cfg.Model,
			Messages: messages,
			Tools:    tools,
		}

		bodyBytes, err := json.Marshal(reqBody)
		if err != nil {
			return "", fmt.Errorf("序列化请求失败: %w", err)
		}

		log.Printf("[AI] 请求 %s (round %d), 消息数: %d", cfg.Name, round+1, len(messages))

		client := &http.Client{Timeout: 120 * time.Second}
		req, err := http.NewRequest("POST", url, bytes.NewReader(bodyBytes))
		if err != nil {
			return "", fmt.Errorf("创建请求失败: %w", err)
		}
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+cfg.APIKey)

		resp, err := client.Do(req)
		if err != nil {
			return "", fmt.Errorf("请求失败: %w", err)
		}
		defer resp.Body.Close()

		respBody, err := io.ReadAll(resp.Body)
		if err != nil {
			return "", fmt.Errorf("读取响应失败: %w", err)
		}

		if resp.StatusCode != http.StatusOK {
			return "", fmt.Errorf("AI 返回错误 (%d): %s", resp.StatusCode, string(respBody))
		}

		var chatResp chatResponse
		if err := json.Unmarshal(respBody, &chatResp); err != nil {
			return "", fmt.Errorf("解析响应失败: %w", err)
		}

		if len(chatResp.Choices) == 0 {
			return "", fmt.Errorf("AI 未返回任何内容")
		}

		choice := chatResp.Choices[0]
		msg := choice.Message

		if len(msg.ToolCalls) == 0 {
			return msg.Content, nil
		}

		messages = append(messages, msg)

		for _, tc := range msg.ToolCalls {
			log.Printf("[AI] 工具调用: %s(%s)", tc.Function.Name, tc.Function.Arguments)

			var result string
			switch tc.Function.Name {
			case "query_warp_issue":
				result = executeQueryWarpIssue(tc.Function.Arguments)
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

	return "", fmt.Errorf("AI 工具调用超过最大轮数 (%d)", maxRounds)
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

	jiraCfg, err := db.GetSSOConfig()
	if err != nil {
		return `{"error": "Jira 未配置，请先配置登录凭据"}`
	}

	password, err := db.GetSSOConfigPassword()
	if err != nil {
		return fmt.Sprintf(`{"error": "获取密码失败: %v"}`, err)
	}

	if jiraCfg.BaseURL == "" {
		jiraCfg.BaseURL = "https://jira.transwarp.io"
	}

	cfg := jira.Config{
		BaseURL:  jiraCfg.BaseURL,
		Username: jiraCfg.Username,
		Password: password,
	}

	issue, err := jira.GetIssue(cfg, issueKey)
	if err != nil {
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

func stripHTMLTags(html string) string {
	var result strings.Builder
	inTag := false
	for _, r := range html {
		if r == '<' {
			inTag = true
			continue
		}
		if r == '>' {
			inTag = false
			continue
		}
		if !inTag {
			result.WriteRune(r)
		}
	}
	return strings.TrimSpace(result.String())
}
