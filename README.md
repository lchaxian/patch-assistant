# Patch Assistant

智能 Patch 发布通知邮件汇总与分析工具。

通过 IMAP 协议同步企业邮箱中的 Patch 发布通知邮件，自动解析产品/版本/类型等信息，并支持 AI 驱动的智能分析，结合 JIRA 工单和 Wiki 文档自动查询，帮助运维和研发团队高效掌握 Patch 动态并生成测试案例。

## ✨ 功能特性

- **Patch 邮件自动解析** — 从 Patch 发布通知邮件中提取产品、版本、类型（预览/通用/定向）、日期、序号等关键信息，按产品分组展示
- **IMAP 邮箱同步** — 支持多账户 IMAP 连接，兼容腾讯企业邮箱等中文编码（GBK/GB2312/GB18030）
- **AI 智能分析** — 对接 OpenAI 兼容接口（DeepSeek、通义千问等），自动生成 Patch 调整摘要、影响范围、注意事项
- **JIRA 工单联动** — AI 分析时自动识别 WARP 工单编号，通过 SSO 认证查询 JIRA 工单详情，结合工单上下文提供更准确的分析
- **Wiki 文档集成** — 自动搜索 Confluence Wiki 上与 WARP 工单关联的文档和附件（测试 SQL、技术方案、配置文件等），附件原文直接输出
- **测试案例自动生成** — AI 根据 JIRA 工单描述和 Wiki 文档内容，为每个调整项生成结构化测试案例（前置条件、测试步骤、预期结果、验证 SQL）
- **自定义提示词** — 支持编辑 AI 汇总提示词，适配不同团队的分析需求
- **数据安全** — 密码使用 AES-256-GCM 加密存储，加密密钥本地文件隔离

## 🏗️ 项目结构

```
.
├── main.go                  # 入口，HTTP 路由与静态文件服务
├── internal/
│   ├── db/
│   │   ├── crypto.go        # AES-256-GCM 加解密
│   │   └── db.go            # SQLite 数据库操作 + 默认提示词
│   ├── handler/
│   │   └── handler.go       # API 处理器
│   ├── jira/
│   │   └── client.go        # JIRA SSO 认证与工单查询
│   ├── model/
│   │   └── model.go         # 数据模型定义
│   ├── service/
│   │   ├── ai.go            # AI 汇总服务（Function Calling + Wiki/JIRA 工具）
│   │   ├── imap.go          # IMAP 邮件同步
│   │   └── patch.go         # Patch 解析与匹配
│   └── wiki/
│       └── client.go        # Confluence Wiki 搜索、页面获取、附件下载
├── cmd/
│   ├── jira-mcp/
│   │   └── main.go          # JIRA MCP Server（stdio JSON-RPC）
│   └── wiki_test/
│       └── main.go          # Wiki 集成测试工具
├── web/
│   ├── src/
│   │   ├── App.jsx          # 应用入口与侧边栏
│   │   ├── api/index.js     # API 请求封装
│   │   ├── index.css        # 全局样式
│   │   └── pages/
│   │       ├── PatchSummary.jsx   # Patch 汇总页（核心）
│   │       ├── Settings.jsx       # 设置页（账户/Jira/AI）
│   │       └── SetupWizard.jsx    # 首次配置向导
│   ├── package.json
│   └── vite.config.js
├── build-linux.sh           # Linux 打包脚本
├── build-windows.sh         # Windows 打包脚本
└── go.mod
```

## 🚀 快速开始

### 环境要求

- Go 1.21+
- Node.js 18+ & npm
- CGO 依赖（SQLite 驱动需要 gcc）

### 本地开发

```bash
# 1. 克隆项目
git clone https://github.com/lchaxian/patch-assistant.git
cd patch-assistant

# 2. 构建前端
cd web
npm install
npm run build
cd ..

# 3. 启动后端
go run .

# 4. 浏览器访问
# http://localhost:8080
```

首次访问会进入配置向导，引导添加邮箱账户、Jira 凭据和 Wiki 地址。

### 编译打包

```bash
# Linux amd64
./build-linux.sh

# Windows amd64（交叉编译）
./build-windows.sh
```

打包产物在 `dist/` 目录，包含可执行文件和启动脚本。

## 📖 使用说明

### Patch 汇总

主页面自动展示已同步的 Patch 通知，支持按邮箱账户和时间范围筛选：

- **本周** — 显示本周收到的 Patch
- **本年** — 显示本年所有 Patch
- **自定义** — 指定起止日期

点击「同步刷新」拉取最新邮件并更新汇总。

### AI 分析

在 Patch 列表中点击「AI 分析」按钮，AI 将：

1. 读取邮件全文
2. 自动识别 WARP 工单编号并查询 JIRA 详情
3. 搜索 Wiki 上关联的文档和附件，自动获取正文与文本类附件内容
4. 生成结构化的 Patch 调整摘要，包含：
   - Patch 基本信息（产品/版本/类型/日期）
   - 调整内容
   - 影响范围
   - 注意事项
   - Wiki 相关信息（附件原文直接输出，如 SQL 脚本、properties 配置）
   - 测试案例（关联 WARP、前置条件、测试步骤、预期结果、验证 SQL）

### 设置

- **邮箱账户** — 管理 IMAP 邮箱连接，支持连接测试和手动同步
- **Jira 配置** — 配置 SSO 用户名/密码，AI 分析时自动查询工单
- **Wiki 配置** — 配置 Confluence 地址，AI 分析时自动搜索关联文档和附件
- **AI 配置** — 添加 OpenAI 兼容的 AI 服务（如 DeepSeek、通义千问），编辑汇总提示词

### JIRA MCP Server

项目附带一个 JIRA MCP Server，可作为 AI 工具链的一部分：

```bash
# 编译
go build -o jira-mcp ./cmd/jira-mcp/

# 运行（stdio JSON-RPC）
./jira-mcp
```

支持 `query_warp_issue` 工具，供 AI Agent 查询 WARP 工单详情。

### Wiki 测试工具

用于验证 Confluence Wiki 连接和搜索功能：

```bash
# 编译
go build -o wiki_test ./cmd/wiki_test/

# 运行
./wiki_test
```

## 🔧 技术栈

| 层级 | 技术 |
|------|------|
| 后端 | Go, Gin, SQLite (modernc.org/sqlite) |
| 前端 | React, Vite, React Router, Lucide Icons |
| 邮件 | IMAP (go-imap), GBK 编码支持 |
| AI | OpenAI 兼容 API, Function Calling |
| JIRA | SSO 认证, REST API |
| Wiki | Confluence REST API, CQL 搜索 |
| 安全 | AES-256-GCM 加密存储 |

## ⚠️ 注意事项

- `encryption.key` 是加密主密钥，**切勿泄露**，丢失后将无法解密已存储的密码
- 数据库 `mail-summary.db` 中存储了加密后的凭据，同样不应公开
- 默认监听端口 `8080`，可通过代码修改
- JIRA SSO 认证地址默认配置为 `https://erp.transwarp.io`，可在设置页修改
- Wiki 地址默认配置为 `https://wiki.transwarp.io`，可在设置页修改

## 📄 License

MIT
