package service

import (
	"log"
	"regexp"
	"strings"

	"github.com/lchaxian/patch-assistant/internal/db"
	"github.com/lchaxian/patch-assistant/internal/model"
)

var patchPattern = regexp.MustCompile(`(预览|通用|定向)?Patch发布通知[-—]\s*【Patch-([^-]+)-([^-]+)-(\d{8})-(\d+)】`)

// ParsePatchSubject 从邮件标题解析 Patch 信息
func ParsePatchSubject(subject string) *model.PatchInfo {
	matches := patchPattern.FindStringSubmatch(subject)
	if matches == nil {
		return parsePatchSubjectLoose(subject)
	}

	patchType := matches[1]
	if patchType == "" {
		patchType = "通用"
	}

	return &model.PatchInfo{
		Subject: subject,
		Type:    patchType,
		Product: matches[2],
		Version: matches[3],
		Date:    matches[4],
		Seq:     matches[5],
	}
}

func parsePatchSubjectLoose(subject string) *model.PatchInfo {
	if !strings.Contains(subject, "Patch") || !strings.Contains(subject, "【") {
		return nil
	}

	patchType := "通用"
	if strings.Contains(subject, "预览") {
		patchType = "预览"
	} else if strings.Contains(subject, "定向") {
		patchType = "定向"
	}

	bracketRe := regexp.MustCompile(`【Patch-([^】]+)】`)
	bracketMatches := bracketRe.FindStringSubmatch(subject)
	if bracketMatches == nil {
		return &model.PatchInfo{
			Subject: subject,
			Type:    patchType,
		}
	}

	parts := strings.Split(bracketMatches[1], "-")
	info := &model.PatchInfo{
		Subject: subject,
		Type:    patchType,
	}

	if len(parts) >= 2 {
		info.Product = parts[1]
	}
	if len(parts) >= 3 {
		info.Version = parts[2]
	}
	if len(parts) >= 4 {
		if len(parts[3]) == 8 {
			info.Date = parts[3]
		}
	}
	if len(parts) >= 5 {
		info.Seq = parts[4]
	}

	return info
}

// BuildPatchSummary 从邮件列表构建 Patch 汇总
func BuildPatchSummary(mails []model.MailItem, timeRange string) *model.PatchSummaryResponse {
	resp := &model.PatchSummaryResponse{
		Range:     timeRange,
		ByProduct: make(map[string]int),
		ByType:    make(map[string]int),
	}

	for _, m := range mails {
		info := ParsePatchSubject(m.Subject)
		if info == nil {
			continue
		}
		info.MailID = m.ID
		info.AccountID = m.AccountID
		info.MailDate = m.Date
		info.FromName = m.FromName
		info.FromAddr = m.From

		resp.Patches = append(resp.Patches, *info)
		resp.ByProduct[info.Product]++
		resp.ByType[info.Type]++
	}

	resp.TotalCount = len(resp.Patches)
	return resp
}

// ParseAndSaveNewPatchMails 增量解析未入库的 Patch 邮件标题并保存
func ParseAndSaveNewPatchMails(accountID int64) {
	mails, err := db.GetUnparsedPatchMails(accountID)
	if err != nil {
		log.Printf("[Patch] 获取未解析邮件失败: %v", err)
		return
	}
	if len(mails) == 0 {
		return
	}
	count := db.ParseAndSavePatchInfos(mails, ParsePatchSubject)
	if count > 0 {
		log.Printf("[Patch] 新解析 %d 封 Patch 邮件", count)
	}
}
