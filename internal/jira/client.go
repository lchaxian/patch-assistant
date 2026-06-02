package jira

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// Config JIRA 登录配置
type Config struct {
	BaseURL  string `json:"base_url"`
	LoginURL string `json:"login_url"`
	Username string `json:"username"`
	Password string `json:"password"`
}

// Issue JIRA Issue 数据结构
type Issue struct {
	Key    string      `json:"key"`
	URL    string      `json:"url"`
	Fields IssueFields `json:"fields"`
}

// IssueFields JIRA Issue 字段
type IssueFields struct {
	Summary     string           `json:"summary"`
	Description string           `json:"description"`
	Status      StatusField      `json:"status"`
	IssueType   IssueTypeField   `json:"issuetype"`
	Priority    PriorityField    `json:"priority"`
	Assignee    UserField        `json:"assignee"`
	Reporter    UserField        `json:"reporter"`
	Created     string           `json:"created"`
	Updated     string           `json:"updated"`
	Labels      []string         `json:"labels"`
	Components  []ComponentField `json:"components"`
	Comments    *Comments        `json:"comment,omitempty"`
}

type StatusField struct {
	Name string `json:"name"`
}

type IssueTypeField struct {
	Name string `json:"name"`
}

type PriorityField struct {
	Name string `json:"name"`
}

type UserField struct {
	DisplayName string `json:"displayName"`
	Email       string `json:"emailAddress"`
}

type ComponentField struct {
	Name string `json:"name"`
}

type Comments struct {
	Total  int            `json:"total"`
	Values []CommentField `json:"values"`
}

type CommentField struct {
	Body    string    `json:"body"`
	Created string    `json:"created"`
	Author  UserField `json:"author"`
}

// TestAuth 通过 JIRA API 验证用户名密码是否正确
// 用一个不存在的 issue key 请求，根据返回状态码判断：
//   - 401: 用户名或密码错误（唯一表示认证失败的状态码）
//   - 403/404: 凭据有效，403 表示无权限访问该 issue，404 表示 issue 不存在
// JIRA 的安全策略：认证失败返回 401，认证通过后无权限或不存在返回 403/404
func TestAuth(cfg Config) error {
	baseURL := strings.TrimRight(cfg.BaseURL, "/")
	url := fmt.Sprintf("%s/rest/api/2/issue/WARP-99999999", baseURL)

	client := &http.Client{Timeout: 15 * time.Second}
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return fmt.Errorf("创建请求失败: %w", err)
	}
	req.SetBasicAuth(cfg.Username, cfg.Password)
	req.Header.Set("Accept", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("连接 JIRA 失败: %w", err)
	}
	defer resp.Body.Close()

	switch resp.StatusCode {
	case http.StatusUnauthorized:
		// 401 = 唯一表示认证失败的状态码
		return fmt.Errorf("用户名或密码错误")
	case http.StatusNotFound, http.StatusForbidden:
		// 404 = 认证通过，issue 不存在
		// 403 = 认证通过，但无权限访问该 issue（JIRA 安全策略不泄露 issue 是否存在）
		return nil
	default:
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("JIRA 返回 %d: %s", resp.StatusCode, string(body))
	}
}

// GetIssue 通过 issue key 获取 JIRA Issue 详情
func GetIssue(cfg Config, issueKey string) (*Issue, error) {
	baseURL := strings.TrimRight(cfg.BaseURL, "/")
	url := fmt.Sprintf("%s/rest/api/2/issue/%s", baseURL, issueKey)

	client := &http.Client{Timeout: 30 * time.Second}
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	req.SetBasicAuth(cfg.Username, cfg.Password)
	req.Header.Set("Accept", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("JIRA API 返回 %d: %s", resp.StatusCode, string(body))
	}

	var rawIssue struct {
		Key    string          `json:"key"`
		Fields json.RawMessage `json:"fields"`
	}
	if err := json.Unmarshal(body, &rawIssue); err != nil {
		return nil, fmt.Errorf("parse issue: %w", err)
	}

	var fields IssueFields
	if err := json.Unmarshal(rawIssue.Fields, &fields); err != nil {
		return nil, fmt.Errorf("parse fields: %w", err)
	}

	comments, _ := getComments(baseURL, cfg, issueKey)
	fields.Comments = comments

	return &Issue{
		Key:    rawIssue.Key,
		URL:    fmt.Sprintf("%s/browse/%s", baseURL, issueKey),
		Fields: fields,
	}, nil
}

func getComments(baseURL string, cfg Config, issueKey string) (*Comments, error) {
	url := fmt.Sprintf("%s/rest/api/2/issue/%s/comment", baseURL, issueKey)

	client := &http.Client{Timeout: 15 * time.Second}
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	req.SetBasicAuth(cfg.Username, cfg.Password)
	req.Header.Set("Accept", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("comments API returned %d", resp.StatusCode)
	}

	var comments Comments
	if err := json.Unmarshal(body, &comments); err != nil {
		return nil, err
	}
	return &comments, nil
}
