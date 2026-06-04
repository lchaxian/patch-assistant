package wiki

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"
)

// Config Wiki 登录配置
type Config struct {
	BaseURL  string `json:"base_url"`
	Username string `json:"username"`
	Password string `json:"password"`
}

// SearchResult Wiki 搜索结果
type SearchResult struct {
	Results []SearchItem `json:"results"`
}

// SearchItem 单条搜索结果
type SearchItem struct {
	ID       string `json:"id"`
	Title    string `json:"title"`
	Type     string `json:"type"`    // page, attachment, blogpost 等
	URL      string `json:"url"`     // 页面/文件访问链接
	Content  string `json:"content"` // 页面正文或文件内容
	Excerpt  string `json:"excerpt"` // 搜索摘要
}

// PageDetail Wiki 页面详情
type PageDetail struct {
	ID          string         `json:"id"`
	Title       string         `json:"title"`
	Type        string         `json:"type"`
	URL         string         `json:"url"`
	Body        string         `json:"body"`
	Attachments []Attachment   `json:"attachments,omitempty"`
}

// Attachment Wiki 页面附件
type Attachment struct {
	ID       string `json:"id"`
	Title    string `json:"title"`
	FileSize int64  `json:"file_size"`
	MediaType string `json:"media_type"`
	DownloadURL string `json:"download_url"`
	Content  string `json:"content,omitempty"` // 文本类附件的内容
}

// --- Confluence REST API 响应结构 ---

type confSearchResponse struct {
	Results []confSearchResult `json:"results"`
	Size    int                `json:"size"`
}

type confSearchResult struct {
	Content              confSearchContent      `json:"content"`
	Title                string                 `json:"title"`   // 带高亮标记 @@hl@@...@@endhl@@
	URL                  string                 `json:"url"`
	Excerpt              string                 `json:"excerpt"`
	ResultParentContainer *confContainerInfo    `json:"resultParentContainer,omitempty"`
	LastModified         string                 `json:"lastModified"`
}

type confSearchContent struct {
	ID     string             `json:"id"`
	Type   string             `json:"type"`
	Title  string             `json:"title"`  // 干净标题，无高亮标记
	Status string             `json:"status"`
	Links  confContentLinks   `json:"_links"`
}

type confContentLinks struct {
	WebUI string `json:"webui"`
	Self  string `json:"self"`
}

type confContainerInfo struct {
	Title      string `json:"title"`
	DisplayURL string `json:"displayUrl"`
}

// confPageResponse 获取页面详情的响应
type confPageResponse struct {
	ID    string `json:"id"`
	Type  string `json:"type"`
	Title string `json:"title"`
	Body  struct {
		Storage struct {
			Value string `json:"value"`
		} `json:"storage"`
		View struct {
			Value string `json:"value"`
		} `json:"view"`
	} `json:"body"`
	Links struct {
		WebUI string `json:"webui"`
	} `json:"_links"`
}

// confAttachmentListResponse 附件列表响应
type confAttachmentListResponse struct {
	Results []confAttachmentItem `json:"results"`
	Size    int                  `json:"size"`
}

type confAttachmentItem struct {
	ID       string `json:"id"`
	Title    string `json:"title"`
	Metadata struct {
		FileSize  int64  `json:"fileSize"`
		MediaType string `json:"mediaType"`
	} `json:"metadata"`
	Links struct {
		Download string `json:"download"`
		WebUI    string `json:"webui"`
	} `json:"_links"`
}

// 预览链接正则：preview=/pageId/attachmentId/filename
var previewURLRegex = regexp.MustCompile(`preview=/(\d+)/(\d+)/([^&\s]+)`)

// doRequest 发起带 Basic Auth 的 HTTP 请求
func doRequest(cfg Config, method, rawURL string) ([]byte, http.Header, int, error) {
	client := &http.Client{Timeout: 30 * time.Second}
	req, err := http.NewRequest(method, rawURL, nil)
	if err != nil {
		return nil, nil, 0, fmt.Errorf("创建请求失败: %w", err)
	}
	req.SetBasicAuth(cfg.Username, cfg.Password)
	req.Header.Set("Accept", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return nil, nil, 0, fmt.Errorf("请求失败: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 500000))
	if err != nil {
		return nil, nil, resp.StatusCode, fmt.Errorf("读取响应失败: %w", err)
	}
	return body, resp.Header, resp.StatusCode, nil
}

// doDownloadRequest 下载二进制/文本内容
func doDownloadRequest(cfg Config, rawURL string) ([]byte, string, int, error) {
	client := &http.Client{Timeout: 30 * time.Second}
	req, err := http.NewRequest("GET", rawURL, nil)
	if err != nil {
		return nil, "", 0, fmt.Errorf("创建下载请求失败: %w", err)
	}
	req.SetBasicAuth(cfg.Username, cfg.Password)

	resp, err := client.Do(req)
	if err != nil {
		return nil, "", 0, fmt.Errorf("下载请求失败: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, "", resp.StatusCode, fmt.Errorf("下载返回 %d", resp.StatusCode)
	}

	contentType := resp.Header.Get("Content-Type")
	data, err := io.ReadAll(io.LimitReader(resp.Body, 500000))
	if err != nil {
		return nil, contentType, resp.StatusCode, fmt.Errorf("读取下载内容失败: %w", err)
	}
	return data, contentType, resp.StatusCode, nil
}

// TestAuth 验证 Wiki 凭据是否有效
func TestAuth(cfg Config) error {
	baseURL := strings.TrimRight(cfg.BaseURL, "/")
	testURL := fmt.Sprintf("%s/rest/api/user/current", baseURL)

	_, _, statusCode, err := doRequest(cfg, "GET", testURL)
	if err != nil {
		return fmt.Errorf("连接 Wiki 失败: %w", err)
	}

	switch statusCode {
	case http.StatusUnauthorized:
		return fmt.Errorf("用户名或密码错误")
	case http.StatusOK:
		return nil
	default:
		// 有些 Confluence 版本 /user/current 返回 403，尝试搜索接口
		searchURL := baseURL + "/rest/api/search?cql=type%3Dpage&limit=1"
		_, _, statusCode2, _ := doRequest(cfg, "GET", searchURL)
		if statusCode2 == http.StatusUnauthorized {
			return fmt.Errorf("用户名或密码错误")
		}
		return nil
	}
}

// SearchWiki 搜索 Wiki 内容
// 搜索关键字为 WARP 号（如 WARP-138971）
// 搜索策略：
//   1. 先搜索标题包含 WARP 号的附件（CQL: title ~ "WARP-xxx" AND type = "attachment"）
//   2. 再搜索标题包含 WARP 号的页面（CQL: title ~ "WARP-xxx" AND type in ("page","blogpost")）
// 对 attachment 类型，自动下载文本类文件内容
// 对 page 类型，获取页面正文和附件列表
// 同名附件去重，只保留最新版本
func SearchWiki(cfg Config, query string) (*SearchResult, error) {
	baseURL := strings.TrimRight(cfg.BaseURL, "/")
	warpKey := strings.ToUpper(query)

	result := &SearchResult{}
	seenFiles := make(map[string]bool) // 文件名去重

	// --- 第一轮：搜索附件 ---
	attCQL := fmt.Sprintf(`title ~ "%s" AND type = "attachment"`, query)
	attResults, err := searchByCQL(cfg, baseURL, attCQL, 10)
	if err != nil {
		log.Printf("[Wiki] 搜索附件失败: %v", err)
	} else {
		for _, item := range attResults {
			title := item.Content.Title
			// 相关度过滤
			if !strings.Contains(strings.ToUpper(title), warpKey) {
				continue
			}
			// 同名附件去重
			if seenFiles[title] {
				continue
			}
			seenFiles[title] = true

			// 构建附件 URL（使用预览链接）
			attURL := ""
			if item.Content.Links.WebUI != "" {
				attURL = baseURL + item.Content.Links.WebUI
			} else if item.URL != "" {
				attURL = baseURL + item.URL
			}

			si := SearchItem{
				ID:      item.Content.ID,
				Title:   title,
				Type:    item.Content.Type,
				URL:     attURL,
				Excerpt: item.Excerpt,
			}

			// 父页面信息
			if item.ResultParentContainer != nil && item.ResultParentContainer.Title != "" {
				si.Excerpt = fmt.Sprintf("所属页面: %s", item.ResultParentContainer.Title)
			}

			// 下载附件内容
			content, downloadErr := downloadAttachmentFromSearchResult(cfg, baseURL, item)
			if downloadErr != nil {
				log.Printf("[Wiki] 下载附件 %s (%s) 失败: %v", item.Content.ID, title, downloadErr)
			} else if content != "" {
				si.Content = truncateText(content, 5000)
			}

			result.Results = append(result.Results, si)
		}
	}

	// --- 第二轮：搜索页面 ---
	// 无关页面关键词：标题包含这些词的页面正文通常与 Patch 分析无关，但其附件（如 SQL 文件）可能仍有价值
	irrelevantPageKeywords := []string{"测试报告", "测试用例报告", "自动化测试报告", "回归测试报告", "集成测试报告", "性能测试报告", "压力测试报告", "验收测试报告"}
	isIrrelevantPage := func(title string) bool {
		for _, kw := range irrelevantPageKeywords {
			if strings.Contains(title, kw) {
				return true
			}
		}
		return false
	}

	pageCQL := fmt.Sprintf(`title ~ "%s" AND type in ("page","blogpost")`, query)
	pageResults, err := searchByCQL(cfg, baseURL, pageCQL, 5)
	if err != nil {
		log.Printf("[Wiki] 搜索页面失败: %v", err)
	} else {
		for _, item := range pageResults {
			title := item.Content.Title
			if !strings.Contains(strings.ToUpper(title), warpKey) {
				continue
			}

			pageURL := ""
			if item.Content.Links.WebUI != "" {
				pageURL = baseURL + item.Content.Links.WebUI
			} else if item.URL != "" {
				pageURL = baseURL + item.URL
			}

			si := SearchItem{
				ID:      item.Content.ID,
				Title:   title,
				Type:    item.Content.Type,
				URL:     pageURL,
				Excerpt: item.Excerpt,
			}

			irrelevant := isIrrelevantPage(title)

			// 获取页面正文和附件（即使无关页面也要获取附件，SQL 文件可能在上面）
			pageDetail, pageErr := GetPageContent(cfg, item.Content.ID)
			if pageErr != nil {
				log.Printf("[Wiki] 获取页面 %s 正文失败: %v", item.Content.ID, pageErr)
			} else {
				// 无关页面跳过正文，只保留标题包含 WARP 号的附件
				if pageDetail.Body != "" && !irrelevant {
					si.Content = truncateText(StripHTMLTags(pageDetail.Body), 5000)
				}
				if irrelevant {
					log.Printf("[Wiki] 无关页面跳过正文: %s", title)
				}
				// 无关页面仅保留标题包含 WARP 号的附件；相关页面保留所有文本附件
				for _, att := range pageDetail.Attachments {
					if att.Content == "" || seenFiles[att.Title] {
						continue
					}
					if irrelevant && !strings.Contains(strings.ToUpper(att.Title), warpKey) {
						continue
					}
					seenFiles[att.Title] = true
					si.Content += fmt.Sprintf("\n\n--- 附件: %s ---\n%s", att.Title, truncateText(att.Content, 5000))
				}
			}

			result.Results = append(result.Results, si)
		}
	}

	log.Printf("[Wiki] 搜索 '%s' 共返回 %d 条结果", query, len(result.Results))
	return result, nil
}

// searchByCQL 执行 CQL 搜索，返回原始结果
func searchByCQL(cfg Config, baseURL, cql string, limit int) ([]confSearchResult, error) {
	params := url.Values{}
	params.Set("cql", cql)
	params.Set("start", "0")
	params.Set("limit", fmt.Sprintf("%d", limit))
	params.Set("includeArchivedSpaces", "false")
	searchURL := fmt.Sprintf("%s/rest/api/search?%s", baseURL, params.Encode())

	log.Printf("[Wiki] CQL 搜索: %s", cql)

	body, _, statusCode, err := doRequest(cfg, "GET", searchURL)
	if err != nil {
		return nil, err
	}

	if statusCode == http.StatusUnauthorized {
		return nil, fmt.Errorf("认证失败")
	}
	if statusCode != http.StatusOK {
		return nil, fmt.Errorf("搜索返回 %d: %s", statusCode, string(body))
	}

	var confResp confSearchResponse
	if err := json.Unmarshal(body, &confResp); err != nil {
		return nil, fmt.Errorf("解析搜索结果失败: %w", err)
	}

	return confResp.Results, nil
}

// downloadAttachmentFromSearchResult 从搜索结果中提取下载 URL 并下载附件
// 优先使用预览链接中的 pageId 和 filename 构建下载路径
func downloadAttachmentFromSearchResult(cfg Config, baseURL string, item confSearchResult) (string, error) {
	// 尝试从 webui URL 解析预览链接
	webui := item.Content.Links.WebUI
	if webui == "" {
		webui = item.URL
	}

	// 从预览链接中提取 pageId 和 filename
	matches := previewURLRegex.FindStringSubmatch(webui)
	if len(matches) >= 4 {
		pageID := matches[1]
		filename := matches[3]
		downloadURL := fmt.Sprintf("%s/download/attachments/%s/%s", baseURL, pageID, url.PathEscape(filename))
		return DownloadAttachmentByURL(cfg, downloadURL)
	}

	// 备用：通过 attachment ID 获取元数据再下载
	return DownloadAttachmentByID(cfg, item.Content.ID)
}

// GetPageContent 获取 Wiki 页面详情（正文 + 附件列表）
func GetPageContent(cfg Config, pageID string) (*PageDetail, error) {
	baseURL := strings.TrimRight(cfg.BaseURL, "/")
	// expand=body.storage 获取页面 HTML 正文
	apiURL := fmt.Sprintf("%s/rest/api/content/%s?expand=body.storage", baseURL, pageID)

	log.Printf("[Wiki] 获取页面内容: %s", apiURL)

	body, _, statusCode, err := doRequest(cfg, "GET", apiURL)
	if err != nil {
		return nil, err
	}
	if statusCode != http.StatusOK {
		return nil, fmt.Errorf("获取页面返回 %d: %s", statusCode, string(body))
	}

	var pageResp confPageResponse
	if err := json.Unmarshal(body, &pageResp); err != nil {
		return nil, fmt.Errorf("解析页面内容失败: %w", err)
	}

	detail := &PageDetail{
		ID:    pageResp.ID,
		Title: pageResp.Title,
		Type:  pageResp.Type,
		Body:  pageResp.Body.Storage.Value,
	}

	// 页面 URL
	if pageResp.Links.WebUI != "" {
		detail.URL = baseURL + pageResp.Links.WebUI
	} else {
		detail.URL = fmt.Sprintf("%s/pages/viewpage.action?pageId=%s", baseURL, pageID)
	}

	// 获取附件列表
	attachments, err := GetPageAttachments(cfg, pageID)
	if err != nil {
		log.Printf("[Wiki] 获取页面 %s 附件列表失败: %v", pageID, err)
	} else {
		detail.Attachments = attachments
	}

	return detail, nil
}

// GetPageAttachments 获取页面附件列表，并下载文本类附件内容
func GetPageAttachments(cfg Config, pageID string) ([]Attachment, error) {
	baseURL := strings.TrimRight(cfg.BaseURL, "/")
	apiURL := fmt.Sprintf("%s/rest/api/content/%s/child/attachment?limit=50", baseURL, pageID)

	log.Printf("[Wiki] 获取附件列表: %s", apiURL)

	body, _, statusCode, err := doRequest(cfg, "GET", apiURL)
	if err != nil {
		return nil, err
	}
	if statusCode != http.StatusOK {
		return nil, fmt.Errorf("获取附件列表返回 %d: %s", statusCode, string(body))
	}

	var attResp confAttachmentListResponse
	if err := json.Unmarshal(body, &attResp); err != nil {
		return nil, fmt.Errorf("解析附件列表失败: %w", err)
	}

	var attachments []Attachment
	for _, item := range attResp.Results {
		att := Attachment{
			ID:        item.ID,
			Title:     item.Title,
			FileSize:  item.Metadata.FileSize,
			MediaType: item.Metadata.MediaType,
		}

		// 构建下载 URL
		if item.Links.Download != "" {
			att.DownloadURL = baseURL + item.Links.Download
		} else {
			// 备用：使用标准路径 /download/attachments/{pageId}/{filename}
			att.DownloadURL = fmt.Sprintf("%s/download/attachments/%s/%s", baseURL, pageID, url.PathEscape(item.Title))
		}

		// 文本类附件，自动下载内容
		if isTextContent(item.Metadata.MediaType, item.Title) {
			if item.Metadata.FileSize > 0 && item.Metadata.FileSize < 200000 {
				content, err := DownloadAttachmentByURL(cfg, att.DownloadURL)
				if err != nil {
					log.Printf("[Wiki] 下载附件 %s 失败: %v", item.Title, err)
				} else if content != "" {
					att.Content = truncateText(content, 5000)
				}
			}
		}

		attachments = append(attachments, att)
	}

	log.Printf("[Wiki] 页面 %s 共 %d 个附件", pageID, len(attachments))
	return attachments, nil
}

// DownloadAttachmentByID 通过 Confluence REST API 下载附件内容（按 attachment content ID）
func DownloadAttachmentByID(cfg Config, attachmentID string) (string, error) {
	baseURL := strings.TrimRight(cfg.BaseURL, "/")
	// Confluence 标准 API：/rest/api/content/{id}/download (某些版本支持)
	// 也可以先获取 attachment 元数据拿到 download 链接
	apiURL := fmt.Sprintf("%s/rest/api/content/%s?expand=metadata,container,version", baseURL, attachmentID)

	body, _, statusCode, err := doRequest(cfg, "GET", apiURL)
	if err != nil {
		return "", err
	}
	if statusCode != http.StatusOK {
		return "", fmt.Errorf("获取附件元数据返回 %d", statusCode)
	}

	var meta struct {
		ID    string `json:"id"`
		Title string `json:"title"`
		Links struct {
			Download string `json:"download"`
			WebUI    string `json:"webui"`
		} `json:"_links"`
		Container struct {
			ID string `json:"id"`
		} `json:"container"`
	}
	if err := json.Unmarshal(body, &meta); err != nil {
		return "", fmt.Errorf("解析附件元数据失败: %w", err)
	}

	// 构建下载 URL
	downloadURL := ""
	if meta.Links.Download != "" {
		downloadURL = baseURL + meta.Links.Download
	} else if meta.Container.ID != "" {
		downloadURL = fmt.Sprintf("%s/download/attachments/%s/%s", baseURL, meta.Container.ID, url.PathEscape(meta.Title))
	} else {
		return "", fmt.Errorf("无法确定附件下载地址")
	}

	return DownloadAttachmentByURL(cfg, downloadURL)
}

// DownloadAttachmentByURL 通过完整 URL 下载附件内容
func DownloadAttachmentByURL(cfg Config, downloadURL string) (string, error) {
	log.Printf("[Wiki] 下载附件: %s", downloadURL)

	data, contentType, statusCode, err := doDownloadRequest(cfg, downloadURL)
	if err != nil {
		return "", err
	}
	if statusCode != http.StatusOK {
		return "", fmt.Errorf("下载附件返回 %d", statusCode)
	}

	// 检查 Content-Type 和文件名，判断是否为文本类文件
	// Confluence 对 SQL 等文件常返回 application/octet-stream;charset=UTF-8
	// 所以需要同时根据文件名后缀判断
	filename := extractFilename(downloadURL)
	if !isTextContent(contentType, filename) {
		log.Printf("[Wiki] 附件是非文本类型 (%s)，跳过", contentType)
		return "", nil
	}

	content := string(data)
	if len(content) > 20000 {
		content = content[:20000] + "\n...(截断)"
	}

	log.Printf("[Wiki] 下载附件成功, 大小: %d, 类型: %s", len(data), contentType)
	return content, nil
}

// DownloadAttachmentFromPreviewURL 从预览链接中解析并下载附件
// 预览链接格式：/pages/viewpage.action?pageId=138278371&preview=/138278371/138278864/10_WARP-138971.sql
func DownloadAttachmentFromPreviewURL(cfg Config, previewURL string) (string, error) {
	matches := previewURLRegex.FindStringSubmatch(previewURL)
	if len(matches) < 4 {
		return "", fmt.Errorf("无法解析预览链接: %s", previewURL)
	}

	pageID := matches[1]
	filename := matches[3]

	baseURL := strings.TrimRight(cfg.BaseURL, "/")
	downloadURL := fmt.Sprintf("%s/download/attachments/%s/%s", baseURL, pageID, url.PathEscape(filename))

	return DownloadAttachmentByURL(cfg, downloadURL)
}

// FetchWikiPageByURL 从 URL 中提取 pageId 并获取页面内容
// 支持的 URL 格式：
//   - https://wiki.transwarp.io/pages/viewpage.action?pageId=138278371
//   - https://wiki.transwarp.io/display/SPACE/Page+Title
//   - https://wiki.transwarp.io/spaces/SPACE/pages/138278371/Page+Title
func FetchWikiPageByURL(cfg Config, rawURL string) (*PageDetail, error) {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return nil, fmt.Errorf("无效的 URL: %w", err)
	}

	// 尝试从 query 中提取 pageId
	if pageID := parsed.Query().Get("pageId"); pageID != "" {
		return GetPageContent(cfg, pageID)
	}

	// 尝试从路径中提取 /spaces/XXX/pages/12345/... 格式
	pathParts := strings.Split(strings.Trim(parsed.Path, "/"), "/")
	for i, p := range pathParts {
		if p == "pages" && i+1 < len(pathParts) {
			// 检查下一个部分是否是数字
			if isNumeric(pathParts[i+1]) {
				return GetPageContent(cfg, pathParts[i+1])
			}
		}
	}

	return nil, fmt.Errorf("无法从 URL 中提取页面 ID: %s", rawURL)
}

// --- 工具函数 ---

// StripHTMLTags 移除 HTML 标签，返回纯文本（块级标签处自动换行）
func StripHTMLTags(html string) string {
	var result strings.Builder
	var tagBuf strings.Builder
	inTag := false
	for _, r := range html {
		if r == '<' {
			inTag = true
			tagBuf.Reset()
			tagBuf.WriteRune(r)
			continue
		}
		if r == '>' {
			inTag = false
			tagBuf.WriteRune(r)
			tagName := extractTagName(tagBuf.String())
			switch tagName {
			case "p", "div", "br", "li", "h1", "h2", "h3", "h4", "h5", "h6", "hr", "blockquote", "table", "tr":
				result.WriteRune('\n')
			case "td", "th":
				result.WriteRune('	')
			}
			continue
		}
		if inTag {
			tagBuf.WriteRune(r)
			continue
		}
		result.WriteRune(r)
	}
	// 清理连续空行
	text := strings.TrimSpace(result.String())
	lines := strings.Split(text, "\n")
	var cleaned []string
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed != "" {
			cleaned = append(cleaned, trimmed)
		}
	}
	return strings.Join(cleaned, "\n")
}

// extractTagName 从 HTML 标签字符串中提取标签名，如 "<br/>" → "br", "</p>" → "p"
func extractTagName(tag string) string {
	tag = strings.Trim(tag, "<>")
	tag = strings.TrimPrefix(tag, "/")
	if idx := strings.IndexAny(tag, " 	\n\r/>"); idx > 0 {
		tag = tag[:idx]
	}
	return strings.ToLower(tag)
}

// truncateText 截断文本
func truncateText(text string, maxLen int) string {
	if len(text) <= maxLen {
		return text
	}
	return text[:maxLen] + "\n...(截断)"
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

// extractFilename 从 URL 或路径中提取纯文件名（去除 query string）
// 例如：/download/attachments/123/10_WARP-138971.sql?version=1&api=v2 -> 10_WARP-138971.sql
func extractFilename(rawURL string) string {
	// 去掉 query string
	if idx := strings.Index(rawURL, "?"); idx >= 0 {
		rawURL = rawURL[:idx]
	}
	// 去掉 fragment
	if idx := strings.Index(rawURL, "#"); idx >= 0 {
		rawURL = rawURL[:idx]
	}
	// 取路径最后一段
	parts := strings.Split(rawURL, "/")
	for i := len(parts) - 1; i >= 0; i-- {
		if parts[i] != "" {
			// URL decode
			if decoded, err := url.PathUnescape(parts[i]); err == nil {
				return decoded
			}
			return parts[i]
		}
	}
	return rawURL
}

// isTextContent 判断 Content-Type 或文件名是否为文本类
// Confluence 对 SQL、properties 等文件常返回 application/octet-stream
// 因此需要同时结合文件名后缀判断
func isTextContent(contentType string, filename string) bool {
	ct := strings.ToLower(contentType)

	// 明确的文本类 Content-Type
	textPrefixes := []string{
		"text/",
		"application/json",
		"application/xml",
		"application/sql",
		"application/javascript",
		"application/x-sh",
		"application/x-sql",
		"application/x-properties",
	}
	for _, prefix := range textPrefixes {
		if strings.HasPrefix(ct, prefix) {
			return true
		}
	}

	// Confluence 返回 application/octet-stream 时，根据文件名后缀判断
	if strings.HasPrefix(ct, "application/octet-stream") {
		return isTextFilename(filename)
	}

	// 其他 Content-Type 中包含文本关键字
	textKeywords := []string{"text", "sql", "xml", "json", "csv", "properties"}
	for _, kw := range textKeywords {
		if strings.Contains(ct, kw) {
			return true
		}
	}

	// 最终 fallback：如果文件名明确是文本文件，也返回 true
	return isTextFilename(filename)
}

// isTextFilename 根据文件名判断是否为文本文件
func isTextFilename(filename string) bool {
	textExts := []string{".sql", ".txt", ".csv", ".xml", ".json", ".properties", ".yaml", ".yml", ".md", ".sh", ".bash", ".conf", ".cfg", ".ini", ".log", ".py", ".js", ".java", ".go"}
	lower := strings.ToLower(filename)
	for _, ext := range textExts {
		if strings.HasSuffix(lower, ext) {
			return true
		}
	}
	return false
}
