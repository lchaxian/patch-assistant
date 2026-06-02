package service

import (
	"encoding/base64"
	"fmt"
	"io"
	"log"
	"mime"
	"mime/multipart"
	"mime/quotedprintable"
	"net/mail"
	"strings"
	"time"

	"github.com/emersion/go-imap"
	"github.com/emersion/go-imap/client"
	"golang.org/x/text/encoding/simplifiedchinese"
	"golang.org/x/text/transform"

	"github.com/lchaxian/patch-assistant/internal/db"
	"github.com/lchaxian/patch-assistant/internal/model"
)

// decodeHeader 解码 RFC 2047 编码的邮件头部字段
func decodeHeader(s string) string {
	if s == "" {
		return s
	}
	fixed := strings.ReplaceAll(s, "?==?", "?= =?")
	dec := new(mime.WordDecoder)
	decoded, err := dec.DecodeHeader(fixed)
	if err == nil && decoded != "" {
		return decoded
	}
	return manualDecodeRFC2047(fixed)
}

// manualDecodeRFC2047 手动解码 RFC 2047 编码字符串
func manualDecodeRFC2047(s string) string {
	var result strings.Builder
	i := 0
	for i < len(s) {
		start := strings.Index(s[i:], "=?")
		if start == -1 {
			result.WriteString(s[i:])
			break
		}
		result.WriteString(s[i : i+start])
		i += start

		end := strings.Index(s[i+2:], "?=")
		if end == -1 {
			result.WriteString(s[i:])
			break
		}

		encodedWord := s[i+2 : i+2+end]
		parts := strings.SplitN(encodedWord, "?", 3)
		if len(parts) != 3 {
			result.WriteString(s[i : i+2+end+2])
			i = i + 2 + end + 2
			continue
		}

		charset := parts[0]
		encoding := strings.ToUpper(parts[1])
		encodedText := parts[2]

		var decoded []byte
		var decodeErr error

		switch encoding {
		case "B":
			decoded, decodeErr = base64.StdEncoding.DecodeString(encodedText)
		case "Q":
			qpReader := quotedprintable.NewReader(strings.NewReader(encodedText))
			decoded, decodeErr = io.ReadAll(qpReader)
		default:
			decoded = []byte(encodedText)
		}

		if decodeErr != nil {
			result.WriteString(s[i : i+2+end+2])
		} else {
			decodedStr := decodeBodyContent(decoded, charset)
			result.WriteString(decodedStr)
		}

		i = i + 2 + end + 2
	}
	return result.String()
}

// IMAPConfig IMAP 配置
type IMAPConfig struct {
	Host     string
	Port     int
	Email    string
	Password string
	UseTLS   bool
}

// TestConnection 测试 IMAP 连接
func TestConnection(config IMAPConfig) error {
	addr := fmt.Sprintf("%s:%d", config.Host, config.Port)
	var c *client.Client
	var err error
	if config.UseTLS {
		c, err = client.DialTLS(addr, nil)
	} else {
		c, err = client.Dial(addr)
	}
	if err != nil {
		return fmt.Errorf("连接服务器失败: %w", err)
	}
	defer c.Logout()

	if err := c.Login(config.Email, config.Password); err != nil {
		return fmt.Errorf("登录失败，请检查用户名和密码: %w", err)
	}
	return nil
}

// SyncMails 同步邮件
func SyncMails(accountID int64, days int) (*model.SyncResult, error) {
	sinceDate := time.Now().AddDate(0, 0, -days)
	return SyncMailsSince(accountID, sinceDate)
}

// SyncMailsSince 同步指定日期之后的邮件
func SyncMailsSince(accountID int64, sinceDate time.Time) (*model.SyncResult, error) {
	result := &model.SyncResult{AccountID: accountID}

	acc, err := db.GetAccount(accountID)
	if err != nil {
		return nil, fmt.Errorf("获取账户信息失败: %w", err)
	}

	password, err := db.GetAccountPassword(accountID)
	if err != nil {
		return nil, fmt.Errorf("获取密码失败: %w", err)
	}

	addr := fmt.Sprintf("%s:%d", acc.IMAPHost, acc.IMAPPort)
	var c *client.Client

	if acc.UseTLS {
		c, err = client.DialTLS(addr, nil)
	} else {
		c, err = client.Dial(addr)
	}
	if err != nil {
		_ = db.UpdateAccountStatus(accountID, "error", "连接失败: "+err.Error())
		return nil, fmt.Errorf("连接服务器失败: %w", err)
	}
	defer c.Logout()

	if err := c.Login(acc.Email, password); err != nil {
		_ = db.UpdateAccountStatus(accountID, "error", "登录失败: "+err.Error())
		return nil, fmt.Errorf("登录失败: %w", err)
	}

	folders := []string{"INBOX"}
	for _, folder := range folders {
		count, err := syncFolderSince(c, accountID, folder, sinceDate)
		if err != nil {
			log.Printf("同步文件夹 %s 失败: %v", folder, err)
			continue
		}
		result.NewMails += count
	}

	var total int64
	db.DB.QueryRow(`SELECT COUNT(*) FROM mails WHERE account_id = ?`, accountID).Scan(&total)
	result.TotalMails = int(total)

	_ = db.UpdateAccountSyncTime(accountID)
	return result, nil
}

func syncFolderSince(c *client.Client, accountID int64, folder string, sinceDate time.Time) (int, error) {
	mbox, err := c.Select(folder, false)
	if err != nil {
		return 0, fmt.Errorf("选择文件夹失败: %w", err)
	}

	if mbox.Messages == 0 {
		return 0, nil
	}

	criteria := imap.NewSearchCriteria()
	criteria.Since = sinceDate

	uids, err := c.UidSearch(criteria)
	if err != nil {
		return syncRecentMails(c, accountID, folder, mbox, 200)
	}

	if len(uids) == 0 {
		return 0, nil
	}

	return twoPhaseSync(c, accountID, folder, uids)
}

func syncRecentMails(c *client.Client, accountID int64, folder string, mbox *imap.MailboxStatus, limit uint32) (int, error) {
	start := uint32(1)
	if mbox.Messages > limit {
		start = mbox.Messages - limit + 1
	}

	seqSet := new(imap.SeqSet)
	seqSet.AddRange(start, mbox.Messages)

	var uids []uint32
	messages := make(chan *imap.Message, 10)
	go func() {
		_ = c.Fetch(seqSet, []imap.FetchItem{imap.FetchUid}, messages)
	}()

	for msg := range messages {
		if msg.Uid > 0 {
			uids = append(uids, msg.Uid)
		}
	}

	if len(uids) == 0 {
		return 0, nil
	}

	return twoPhaseSync(c, accountID, folder, uids)
}

// twoPhaseSync 两阶段同步
func twoPhaseSync(c *client.Client, accountID int64, folder string, uids []uint32) (int, error) {
	existingIDs, err := db.GetExistingMessageIDs(accountID, folder)
	if err != nil {
		log.Printf("[WARN] 获取已有邮件ID失败: %v", err)
		existingIDs = make(map[string]bool)
	}

	metadataBatchSize := 100
	var newMails []model.MailItem

	for i := 0; i < len(uids); i += metadataBatchSize {
		end := i + metadataBatchSize
		if end > len(uids) {
			end = len(uids)
		}
		batch := uids[i:end]

		uidSet := new(imap.SeqSet)
		for _, uid := range batch {
			uidSet.AddNum(uid)
		}

		messages := make(chan *imap.Message, 20)
		fetchItems := []imap.FetchItem{
			imap.FetchEnvelope,
			imap.FetchFlags,
			imap.FetchUid,
			imap.FetchRFC822Size,
		}

		go func() {
			_ = c.UidFetch(uidSet, fetchItems, messages)
		}()

		for msg := range messages {
			mailItem := convertMail(accountID, folder, msg)
			if mailItem == nil {
				continue
			}
			if existingIDs[mailItem.MessageID] {
				continue
			}
			newMails = append(newMails, *mailItem)
		}
	}

	if len(newMails) == 0 {
		return 0, nil
	}

	log.Printf("[INFO] 发现 %d 封新邮件，开始拉取正文", len(newMails))

	inserted, err := db.BatchInsertMails(newMails)
	if err != nil {
		log.Printf("[WARN] 批量插入元数据失败: %v", err)
		inserted = 0
		for _, m := range newMails {
			if db.InsertMail(&m) == nil {
				inserted++
			}
		}
	}

	bodyBatchSize := 30
	rfc822Section, _ := imap.ParseBodySectionName("RFC822")
	bodyUpdated := 0

	newMsgIDs := make(map[string]bool)
	for _, m := range newMails {
		newMsgIDs[m.MessageID] = true
	}

	for i := 0; i < len(uids); i += bodyBatchSize {
		end := i + bodyBatchSize
		if end > len(uids) {
			end = len(uids)
		}
		batch := uids[i:end]

		uidSet := new(imap.SeqSet)
		for _, uid := range batch {
			uidSet.AddNum(uid)
		}

		messages := make(chan *imap.Message, 10)
		fetchItems := []imap.FetchItem{
			imap.FetchEnvelope,
			imap.FetchUid,
			rfc822Section.FetchItem(),
		}

		go func() {
			_ = c.UidFetch(uidSet, fetchItems, messages)
		}()

		for msg := range messages {
			if msg.Envelope == nil {
				continue
			}
			msgID := msg.Envelope.MessageId
			if !newMsgIDs[msgID] {
				continue
			}

			bodyText, bodyHTML := extractBody(msg, rfc822Section)
			if bodyText == "" && bodyHTML == "" {
				continue
			}

			_, err := db.DB.Exec(
				`UPDATE mails SET body_text=?, body_html=? WHERE account_id=? AND message_id=? AND folder=?`,
				bodyText, bodyHTML, accountID, msgID, folder)
			if err == nil {
				bodyUpdated++
			}
		}
	}

	log.Printf("[INFO] 正文更新完成: %d/%d", bodyUpdated, inserted)
	return inserted, nil
}

// extractBody 从 IMAP 消息中提取正文
func extractBody(msg *imap.Message, section *imap.BodySectionName) (text string, html string) {
	r := msg.GetBody(section)
	if r == nil {
		for _, literal := range msg.Body {
			if literal != nil {
				r = literal
				break
			}
		}
	}

	if r == nil {
		return "", ""
	}

	data, err := io.ReadAll(r)
	if err != nil {
		return "", ""
	}

	if len(data) == 0 {
		return "", ""
	}

	return parseMIMEBody(string(data))
}

// parseMIMEBody 解析 MIME 格式的邮件正文
func parseMIMEBody(raw string) (textBody, htmlBody string) {
	reader := strings.NewReader(raw)
	msg, err := mail.ReadMessage(reader)
	if err != nil {
		return raw, ""
	}

	mediaType, params, err := mime.ParseMediaType(msg.Header.Get("Content-Type"))
	if err != nil {
		bodyData, _ := io.ReadAll(msg.Body)
		return string(bodyData), ""
	}

	if strings.HasPrefix(mediaType, "multipart/") {
		boundary := params["boundary"]
		if boundary == "" {
			bodyData, _ := io.ReadAll(msg.Body)
			return string(bodyData), ""
		}
		mr := multipart.NewReader(msg.Body, boundary)
		for {
			part, err := mr.NextPart()
			if err != nil {
				break
			}
			partData, err := io.ReadAll(part)
			if err != nil {
				continue
			}

			partMediaType, partParams, _ := mime.ParseMediaType(part.Header.Get("Content-Type"))
			charset := ""
			if partParams != nil {
				charset = partParams["charset"]
			}

			partData = decodeTransferEncoding(partData, part.Header.Get("Content-Transfer-Encoding"))
			content := decodeBodyContent(partData, charset)

			if strings.HasPrefix(partMediaType, "text/html") && htmlBody == "" {
				htmlBody = content
			} else if strings.HasPrefix(partMediaType, "text/plain") && textBody == "" {
				textBody = content
			} else if strings.HasPrefix(partMediaType, "multipart/") {
				nestedBoundary := partParams["boundary"]
				if nestedBoundary != "" {
					nestedMR := multipart.NewReader(strings.NewReader(string(partData)), nestedBoundary)
					extractFromMultipart(nestedMR, &textBody, &htmlBody)
				}
			}
		}
	} else if strings.HasPrefix(mediaType, "text/html") {
		charset := ""
		if params != nil {
			charset = params["charset"]
		}
		bodyData, _ := io.ReadAll(msg.Body)
		bodyData = decodeTransferEncoding(bodyData, msg.Header.Get("Content-Transfer-Encoding"))
		htmlBody = decodeBodyContent(bodyData, charset)
	} else {
		charset := ""
		if params != nil {
			charset = params["charset"]
		}
		bodyData, _ := io.ReadAll(msg.Body)
		bodyData = decodeTransferEncoding(bodyData, msg.Header.Get("Content-Transfer-Encoding"))
		textBody = decodeBodyContent(bodyData, charset)
	}

	return textBody, htmlBody
}

// extractFromMultipart 从嵌套的 multipart 中提取正文
func extractFromMultipart(mr *multipart.Reader, textBody, htmlBody *string) {
	for {
		part, err := mr.NextPart()
		if err != nil {
			break
		}
		partData, err := io.ReadAll(part)
		if err != nil {
			continue
		}

		partMediaType, partParams, _ := mime.ParseMediaType(part.Header.Get("Content-Type"))
		charset := ""
		if partParams != nil {
			charset = partParams["charset"]
		}

		partData = decodeTransferEncoding(partData, part.Header.Get("Content-Transfer-Encoding"))
		content := decodeBodyContent(partData, charset)

		if strings.HasPrefix(partMediaType, "text/html") && *htmlBody == "" {
			*htmlBody = content
		} else if strings.HasPrefix(partMediaType, "text/plain") && *textBody == "" {
			*textBody = content
		} else if strings.HasPrefix(partMediaType, "multipart/") {
			nestedBoundary := partParams["boundary"]
			if nestedBoundary != "" {
				nestedMR := multipart.NewReader(strings.NewReader(string(partData)), nestedBoundary)
				extractFromMultipart(nestedMR, textBody, htmlBody)
			}
		}
	}
}

// decodeTransferEncoding 处理 Content-Transfer-Encoding
func decodeTransferEncoding(data []byte, encoding string) []byte {
	switch strings.ToLower(strings.TrimSpace(encoding)) {
	case "base64":
		decoded, err := base64.StdEncoding.DecodeString(string(data))
		if err != nil {
			cleaned := strings.ReplaceAll(string(data), "\r", "")
			cleaned = strings.ReplaceAll(cleaned, "\n", "")
			decoded, err = base64.StdEncoding.DecodeString(cleaned)
			if err != nil {
				return data
			}
		}
		return decoded
	case "quoted-printable":
		decoded, err := io.ReadAll(quotedprintable.NewReader(strings.NewReader(string(data))))
		if err != nil {
			return data
		}
		return decoded
	default:
		return data
	}
}

// decodeBodyContent 将字节流从指定字符集转换为 UTF-8
func decodeBodyContent(data []byte, charset string) string {
	charsetLower := strings.ToLower(strings.TrimSpace(charset))
	switch charsetLower {
	case "utf-8", "utf8", "":
		return string(data)
	case "gb2312", "gbk", "gb18030":
		reader := transform.NewReader(strings.NewReader(string(data)), simplifiedchinese.GB18030.NewDecoder())
		decoded, err := io.ReadAll(reader)
		if err != nil {
			return string(data)
		}
		return string(decoded)
	case "big5":
		return string(data)
	case "iso-8859-1", "latin1":
		result := make([]rune, len(data))
		for i, b := range data {
			result[i] = rune(b)
		}
		return string(result)
	default:
		return string(data)
	}
}

func convertMail(accountID int64, folder string, msg *imap.Message) *model.MailItem {
	if msg.Envelope == nil {
		return nil
	}

	env := msg.Envelope
	rawSubject := env.Subject
	decodedSubject := decodeHeader(rawSubject)

	mailItem := &model.MailItem{
		AccountID: accountID,
		MessageID: env.MessageId,
		Subject:   decodedSubject,
		Date:      env.Date,
		Size:      int64(msg.Size),
		Folder:    folder,
		IsRead:    false,
		HasAttach: false,
	}

	for _, flag := range msg.Flags {
		if flag == "\\Seen" {
			mailItem.IsRead = true
			break
		}
	}

	if len(env.From) > 0 {
		mailItem.From = env.From[0].Address()
		mailItem.FromName = decodeHeader(env.From[0].PersonalName)
		if mailItem.FromName == "" {
			mailItem.FromName = mailItem.From
		}
	}

	if len(env.To) > 0 {
		var toAddrs []string
		for _, addr := range env.To {
			toAddrs = append(toAddrs, addr.Address())
		}
		mailItem.To = strings.Join(toAddrs, ", ")
	}

	return mailItem
}
