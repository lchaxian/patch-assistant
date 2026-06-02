#!/bin/bash
# 一键打包脚本 — macOS (arm64 + amd64)
set -euo pipefail

PROJECT="patch-assistant"
VERSION=$(date +%Y%m%d%H%M)

# 支持参数选择架构: ./build-mac.sh arm64 | amd64 | all (默认 all)
ARCH="${1:-all}"

build_arch() {
  local GOARCH="$1"
  local ARCH_LABEL="$2"
  local OUTPUT_DIR="dist/${PROJECT}-darwin-${GOARCH}"

  echo ""
  echo "=========================================="
  echo "  Patch助手 — macOS ${ARCH_LABEL} 打包"
  echo "=========================================="

  # 1. 构建前端（仅首次需要）
  echo ""
  echo "[1/4] 构建前端..."
  cd web
  npm install --prefer-offline 2>/dev/null
  npm run build
  cd ..
  echo "      ✔ 前端构建完成 (web/dist/)"

  # 2. 编译 patch-assistant 主程序
  echo ""
  echo "[2/4] 编译 patch-assistant (darwin/${GOARCH})..."
  CGO_ENABLED=0 GOOS=darwin GOARCH=${GOARCH} go build \
    -ldflags '-s -w' \
    -o "${OUTPUT_DIR}/${PROJECT}" .
  echo "      ✔ patch-assistant 编译完成"

  # 3. 编译 jira-mcp
  echo ""
  echo "[3/4] 编译 jira-mcp (darwin/${GOARCH})..."
  CGO_ENABLED=0 GOOS=darwin GOARCH=${GOARCH} go build \
    -ldflags '-s -w' \
    -o "${OUTPUT_DIR}/jira-mcp" ./cmd/jira-mcp/
  echo "      ✔ jira-mcp 编译完成"

  # 4. 打包
  echo ""
  echo "[4/4] 打包..."
  ARCHIVE="dist/${PROJECT}-darwin-${GOARCH}-${VERSION}.tar.gz"

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

适用架构: macOS ${ARCH_LABEL}
打包时间: ${VERSION}
EOF

  tar -czf "${ARCHIVE}" -C dist "${PROJECT}-darwin-${GOARCH}"
  SIZE=$(du -h "${ARCHIVE}" | cut -f1)

  echo "      ✔ 打包完成: ${ARCHIVE} (${SIZE})"
  echo ""
  echo "=========================================="
  echo "  ✔ ${ARCH_LABEL} 打包完成！"
  echo "  产物: ${ARCHIVE}"
  echo "=========================================="
}

if [ "${ARCH}" = "all" ]; then
  build_arch arm64 "Apple Silicon"
  build_arch amd64 "Intel"
else
  case "${ARCH}" in
    arm64) build_arch arm64 "Apple Silicon" ;;
    amd64) build_arch amd64 "Intel" ;;
    *)
      echo "用法: $0 [arm64|amd64|all]"
      echo "  arm64  — Apple Silicon (M1/M2/M3/M4)"
      echo "  amd64  — Intel Mac"
      echo "  all    — 同时打包两种架构 (默认)"
      exit 1
      ;;
  esac
fi
