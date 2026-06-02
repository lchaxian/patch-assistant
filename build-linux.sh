#!/bin/bash
# 一键打包脚本 — Linux amd64
set -euo pipefail

PROJECT="patch-assistant"
VERSION=$(date +%Y%m%d%H%M)
OUTPUT_DIR="dist/${PROJECT}-linux-amd64"

echo "=========================================="
echo "  Patch助手 — Linux amd64 打包"
echo "=========================================="

# 1. 构建前端
echo ""
echo "[1/4] 构建前端..."
cd web
npm install --prefer-offive 2>/dev/null
npm run build
cd ..
echo "      ✔ 前端构建完成 (web/dist/)"

# 2. 构建 patch-assistant 主程序
echo ""
echo "[2/4] 编译 patch-assistant (linux/amd64)..."
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build \
  -ldflags '-s -w' \
  -o "${OUTPUT_DIR}/${PROJECT}" .
echo "      ✔ patch-assistant 编译完成"

# 3. 编译 jira-mcp
echo ""
echo "[3/4] 编译 jira-mcp (linux/amd64)..."
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build \
  -ldflags '-s -w' \
  -o "${OUTPUT_DIR}/jira-mcp" ./cmd/jira-mcp/
echo "      ✔ jira-mcp 编译完成"

# 4. 打包
echo ""
echo "[4/4] 打包..."
ARCHIVE="dist/${PROJECT}-linux-amd64-${VERSION}.tar.gz"

# 写入启动脚本
cat > "${OUTPUT_DIR}/start.sh" << 'EOF'
#!/bin/bash
DIR="$(cd "$(dirname "$0")" && pwd)"
cd "$DIR"
chmod +x patch-assistant jira-mcp 2>/dev/null
echo "启动 Patch助手 服务..."
echo "访问 http://localhost:8080"
echo ""
./patch-assistant
EOF
chmod +x "${OUTPUT_DIR}/start.sh"

# 写入说明
cat > "${OUTPUT_DIR}/README.txt" << EOF
Patch助手 — Patch 发布通知邮件汇总工具
==========================================

功能：
  - 邮箱账户管理（IMAP 同步）
  - Patch 发布通知解析与汇总
  - AI 智能汇总（Function Calling + Jira）
  - Jira/SSO 集成（查询 WARP 工单详情）

使用方法：
  1. 直接运行:  ./start.sh
                或 ./patch-assistant
  2. 浏览器打开: http://localhost:8080
  3. 首次使用时会引导配置邮箱和 Jira 信息

端口：8080
数据库：mail-summary.db（运行时自动创建）
加密密钥：encryption.key（运行时自动创建）

打包时间: ${VERSION}
EOF

tar -czf "${ARCHIVE}" -C dist "${PROJECT}-linux-amd64"
SIZE=$(du -h "${ARCHIVE}" | cut -f1)

echo "      ✔ 打包完成: ${ARCHIVE} (${SIZE})"
echo ""
echo "=========================================="
echo "  ✔ 全部完成！"
echo "  产物: ${ARCHIVE}"
echo "=========================================="
