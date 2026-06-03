package main

import (
	"embed"
	"io"
	"io/fs"
	"log"
	"net/http"
	"strings"

	"github.com/emersion/go-imap"
	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"golang.org/x/text/encoding/simplifiedchinese"
	"golang.org/x/text/transform"

	"github.com/lchaxian/patch-assistant/internal/db"
	"github.com/lchaxian/patch-assistant/internal/handler"
	"github.com/lchaxian/patch-assistant/internal/service"
)

//go:embed web/dist
var webDistFS embed.FS

func main() {
	// 设置 IMAP CharsetReader，支持 GBK/GB2312/GB18030 等中文编码
	imap.CharsetReader = func(charset string, r io.Reader) (io.Reader, error) {
		switch strings.ToLower(charset) {
		case "gb2312", "gbk", "gb18030":
			return transform.NewReader(r, simplifiedchinese.GB18030.NewDecoder()), nil
		case "big5":
			return r, nil
		default:
			return r, nil
		}
	}

	// 初始化数据库
	if err := db.Init("mail-summary.db"); err != nil {
		log.Fatalf("Failed to initialize database: %v", err)
	}
	defer db.Close()

	// 回填已有邮件的 Patch 解析信息
	service.ParseAndSaveNewPatchMails(0)

	r := gin.Default()
	r.RedirectTrailingSlash = false

	// CORS 配置
	r.Use(cors.New(cors.Config{
		AllowOrigins:     []string{"http://localhost:5173", "http://localhost:3000"},
		AllowMethods:     []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowHeaders:     []string{"Origin", "Content-Type", "Authorization"},
		AllowCredentials: true,
	}))

	// 静态文件服务（从 embed.FS 读取）
	distFS, _ := fs.Sub(webDistFS, "web/dist")
	fileServer := http.FileServer(http.FS(distFS))

	r.GET("/", func(c *gin.Context) {
		data, _ := fs.ReadFile(distFS, "index.html")
		c.Data(http.StatusOK, "text/html; charset=utf-8", data)
	})

	r.GET("/assets/*filepath", func(c *gin.Context) {
		fileServer.ServeHTTP(c.Writer, c.Request)
	})
	r.GET("/favicon.svg", func(c *gin.Context) {
		fileServer.ServeHTTP(c.Writer, c.Request)
	})

	// SPA 路由回退
	r.NoRoute(func(c *gin.Context) {
		if c.Request.Method != "GET" || strings.HasPrefix(c.Request.URL.Path, "/api/") {
			c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
			return
		}
		data, _ := fs.ReadFile(distFS, "index.html")
		c.Data(http.StatusOK, "text/html; charset=utf-8", data)
	})

	// API 路由
	api := r.Group("/api")
	{
		accounts := api.Group("/accounts")
		{
			accounts.GET("", handler.ListAccounts)
			accounts.POST("", handler.CreateAccount)
			accounts.GET("/:id", handler.GetAccount)
			accounts.PUT("/:id", handler.UpdateAccount)
			accounts.DELETE("/:id", handler.DeleteAccount)
			accounts.POST("/:id/test", handler.TestAccountConnection)
		}

		mails := api.Group("/mails")
		{
			mails.GET("/summary", handler.GetMailSummary)
			mails.GET("/account/:id", handler.GetAccountMails)
			mails.POST("/sync/:id", handler.SyncAccountMails)
			mails.GET("/:id", handler.GetMailDetail)
		}

		stats := api.Group("/stats")
		{
			stats.GET("/overview", handler.GetOverview)
		}

		api.GET("/patches/summary", handler.GetPatchSummary)

		ai := api.Group("/ai")
		{
			ai.GET("/configs", handler.ListAIConfigs)
			ai.POST("/configs", handler.CreateAIConfig)
			ai.PUT("/configs/:id", handler.UpdateAIConfig)
			ai.DELETE("/configs/:id", handler.DeleteAIConfig)
			ai.PUT("/configs/:id/default", handler.SetDefaultAIConfigHandler)
			ai.POST("/summarize", handler.AISummarize)
			ai.GET("/summary/:id", handler.GetAISummary)
			ai.POST("/summaries/batch", handler.BatchGetAISummaries)
			ai.GET("/prompt", handler.GetAIPrompt)
			ai.PUT("/prompt", handler.SaveAIPrompt)
		}

		jiraCfg := api.Group("/jira-config")
		{
			jiraCfg.GET("", handler.GetJiraConfig)
			jiraCfg.POST("", handler.SaveJiraConfig)
		}

		api.GET("/setup/status", handler.GetSetupStatus)
		api.POST("/setup/complete", handler.CompleteSetup)
	}

	log.Println("Server starting on :8080")
	if err := r.Run(":8080"); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}
