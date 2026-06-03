package main

import (
	"encoding/json"
	"fmt"
	"log"
	"os"

	"github.com/lchaxian/patch-assistant/internal/db"
	"github.com/lchaxian/patch-assistant/internal/wiki"
)

func main() {
	if err := db.Init("mail-summary.db"); err != nil {
		log.Fatalf("初始化数据库失败: %v", err)
	}
	defer db.Close()

	ssoCfg, err := db.GetSSOConfig()
	if err != nil {
		log.Fatalf("获取 SSO 配置失败: %v", err)
	}
	if ssoCfg.Username == "" {
		log.Fatal("SSO 未配置，请先在设置页面配置 Jira/Wiki 凭据")
	}

	password, err := db.GetSSOConfigPassword()
	if err != nil {
		log.Fatalf("获取密码失败: %v", err)
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

	query := "WARP-138971"
	fmt.Printf("搜索 Wiki: %s\n", query)
	fmt.Printf("Wiki URL: %s\n", wikiURL)
	fmt.Println("---")

	result, err := wiki.SearchWiki(cfg, query)
	if err != nil {
		log.Fatalf("搜索失败: %v", err)
	}

	fmt.Printf("找到 %d 条相关结果:\n\n", len(result.Results))
	for i, item := range result.Results {
		fmt.Printf("[%d] ID: %s\n", i+1, item.ID)
		fmt.Printf("    Title: %s\n", item.Title)
		fmt.Printf("    Type: %s\n", item.Type)
		fmt.Printf("    URL: %s\n", item.URL)
		if item.Content != "" {
			preview := item.Content
			if len(preview) > 500 {
				preview = preview[:500] + "\n...(截断)"
			}
			fmt.Printf("    Content:\n%s\n", preview)
		}
		fmt.Println()
	}

	jsonBytes, _ := json.MarshalIndent(result, "", "  ")
	os.Stdout.Write(jsonBytes)
	fmt.Println()
}
