package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/lchaxian/patch-assistant/internal/db"
	"github.com/lchaxian/patch-assistant/internal/jira"
)

type JSONRPCRequest struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

type JSONRPCResponse struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id"`
	Result  interface{}     `json:"result,omitempty"`
	Error   *JSONRPCError   `json:"error,omitempty"`
}

type JSONRPCError struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

type ToolDefinition struct {
	Name        string          `json:"name"`
	Description string          `json:"description"`
	InputSchema ToolInputSchema `json:"inputSchema"`
}

type ToolInputSchema struct {
	Type       string                    `json:"type"`
	Properties map[string]PropertySchema `json:"properties"`
	Required   []string                  `json:"required,omitempty"`
}

type PropertySchema struct {
	Type        string `json:"type"`
	Description string `json:"description"`
}

type toolCallParams struct {
	Name      string          `json:"name"`
	Arguments json.RawMessage `json:"arguments"`
}

func jsonRPCError(id json.RawMessage, code int, msg string) JSONRPCResponse {
	return JSONRPCResponse{
		JSONRPC: "2.0",
		ID:      id,
		Error:   &JSONRPCError{Code: code, Message: msg},
	}
}

func jsonRPCSuccess(id json.RawMessage, result interface{}) JSONRPCResponse {
	return JSONRPCResponse{
		JSONRPC: "2.0",
		ID:      id,
		Result:  result,
	}
}

func main() {
	log.SetFlags(log.Ltime | log.Lmsgprefix)
	log.SetPrefix("[jira-mcp] ")
	log.Printf("starting JIRA MCP server")

	if err := db.Init("mail-summary.db"); err != nil {
		log.Fatalf("Failed to initialize database: %v", err)
	}
	defer db.Close()

	scanner := bufio.NewScanner(os.Stdin)
	log.Printf("MCP server ready, waiting for JSON-RPC messages on stdin")

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}

		var req JSONRPCRequest
		if err := json.Unmarshal([]byte(line), &req); err != nil {
			log.Printf("parse error: %v", err)
			resp := jsonRPCError(nil, -32700, "Parse error: "+err.Error())
			writeJSON(resp)
			continue
		}

		resp := handleRequest(req)
		writeJSON(resp)
	}

	if err := scanner.Err(); err != nil {
		log.Fatalf("stdin error: %v", err)
	}
}

func writeJSON(v interface{}) {
	data, err := json.Marshal(v)
	if err != nil {
		log.Printf("marshal response error: %v", err)
		return
	}
	fmt.Println(string(data))
}

func handleRequest(req JSONRPCRequest) JSONRPCResponse {
	switch req.Method {
	case "initialize":
		return handleInitialize(req)
	case "tools/list":
		return handleToolsList(req)
	case "tools/call":
		return handleToolsCall(req)
	default:
		return jsonRPCError(req.ID, -32601, fmt.Sprintf("Method not found: %s", req.Method))
	}
}

func handleInitialize(req JSONRPCRequest) JSONRPCResponse {
	result := map[string]interface{}{
		"protocolVersion": "2024-11-05",
		"capabilities": map[string]interface{}{
			"tools": map[string]interface{}{},
		},
		"serverInfo": map[string]string{
			"name":    "jira-mcp",
			"version": "1.0.0",
		},
	}
	return jsonRPCSuccess(req.ID, result)
}

func handleToolsList(req JSONRPCRequest) JSONRPCResponse {
	tools := []ToolDefinition{
		{
			Name:        "query_warp_issue",
			Description: "根据 WARP 编号查询 JIRA Issue 详情（https://jira.transwarp.io）。返回 issue 的标题、描述、状态、分配人、报告人、创建时间、更新时间、标签、组件和评论列表。使用界面配置的 SSO/JIRA 账号自动认证。",
			InputSchema: ToolInputSchema{
				Type: "object",
				Properties: map[string]PropertySchema{
					"issue_key": {Type: "string", Description: "WARP 编号，如 WARP-143944"},
				},
				Required: []string{"issue_key"},
			},
		},
	}
	return jsonRPCSuccess(req.ID, map[string]interface{}{
		"tools": tools,
	})
}

func handleToolsCall(req JSONRPCRequest) JSONRPCResponse {
	var params toolCallParams
	if err := json.Unmarshal(req.Params, &params); err != nil {
		return jsonRPCError(req.ID, -32602, fmt.Sprintf("Invalid params: %v", err))
	}

	args := make(map[string]string)
	if len(params.Arguments) > 0 {
		if err := json.Unmarshal(params.Arguments, &args); err != nil {
			return jsonRPCError(req.ID, -32602, fmt.Sprintf("Invalid arguments: %v", err))
		}
	}

	switch params.Name {
	case "query_warp_issue":
		return handleQueryWarpIssue(req, args)
	default:
		return jsonRPCError(req.ID, -32601, fmt.Sprintf("Unknown tool: %s", params.Name))
	}
}

func handleQueryWarpIssue(req JSONRPCRequest, args map[string]string) JSONRPCResponse {
	issueKey := args["issue_key"]
	if issueKey == "" {
		return jsonRPCError(req.ID, -32602, "issue_key is required")
	}

	if !strings.Contains(issueKey, "-") {
		issueKey = "WARP-" + issueKey
	} else {
		issueKey = strings.ToUpper(issueKey)
	}

	cfg, err := db.GetSSOConfig()
	if err != nil {
		return jsonRPCError(req.ID, -32000, "Jira 未配置，请先配置登录凭据")
	}

	password, err := db.GetSSOConfigPassword()
	if err != nil {
		return jsonRPCError(req.ID, -32000, "获取密码失败")
	}

	if cfg.BaseURL == "" {
		cfg.BaseURL = "https://jira.transwarp.io"
	}

	jiraCfg := jira.Config{
		BaseURL:  cfg.BaseURL,
		Username: cfg.Username,
		Password: password,
	}

	issue, err := jira.GetIssue(jiraCfg, issueKey)
	if err != nil {
		return jsonRPCError(req.ID, -32000, fmt.Sprintf("查询 JIRA Issue 失败: %v", err))
	}

	result := map[string]interface{}{
		"key":        issue.Key,
		"url":        issue.URL,
		"summary":    issue.Fields.Summary,
		"status":     issue.Fields.Status.Name,
		"type":       issue.Fields.IssueType.Name,
		"priority":   issue.Fields.Priority.Name,
		"assignee":   issue.Fields.Assignee.DisplayName,
		"reporter":   issue.Fields.Reporter.DisplayName,
		"created":    issue.Fields.Created,
		"updated":    issue.Fields.Updated,
		"labels":     issue.Fields.Labels,
		"description": issue.Fields.Description,
	}

	if len(issue.Fields.Components) > 0 {
		var comps []string
		for _, c := range issue.Fields.Components {
			comps = append(comps, c.Name)
		}
		result["components"] = comps
	}

	if issue.Fields.Comments != nil && len(issue.Fields.Comments.Values) > 0 {
		var comments []map[string]interface{}
		for _, c := range issue.Fields.Comments.Values {
			comments = append(comments, map[string]interface{}{
				"author":  c.Author.DisplayName,
				"created": c.Created,
				"body":    c.Body,
			})
		}
		result["comments"] = comments
		result["comment_count"] = len(comments)
	}

	log.Printf("queried JIRA issue %s: %s", issue.Key, issue.Fields.Summary)
	return jsonRPCSuccess(req.ID, map[string]interface{}{
		"data": result,
	})
}
