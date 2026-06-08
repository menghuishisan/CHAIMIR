#!/usr/bin/env bash
# 建库初始化总入口(本地开发 / 私有化交付用)。
# 依据 docs/总-部署架构设计.md §十:初始化脚本负责编排建库、RLS、角色和初始化数据。
# 业务规则与迁移/授权实现统一委托 backend/cmd/migrate,避免脚本内保留第二套逻辑。
#
# 用法:
#   PG_HOST=127.0.0.1 PG_PORT=5432 PG_DATABASE=chaimir PG_PRIV_USER=postgres PG_PRIV_PASSWORD="<owner口令>" \
#   PG_PASSWORD="<app角色口令>" \
#   bash scripts/db/init.sh
#
# 铁律(CLAUDE.md §6):口令走环境变量,不硬编码;脚本只做编排,不含业务逻辑。
set -euo pipefail

# --- 必需环境变量校验(边界处校验)---
: "${PG_HOST:?需设置 PG_HOST(数据库主机)}"
: "${PG_PORT:?需设置 PG_PORT(数据库端口)}"
: "${PG_DATABASE:?需设置 PG_DATABASE(数据库名)}"
: "${PG_PRIV_USER:?需设置 PG_PRIV_USER(迁移/授权特权用户)}"
: "${PG_PRIV_PASSWORD:?需设置 PG_PRIV_PASSWORD(迁移/授权特权用户口令)}"
: "${PG_PASSWORD:?需设置 PG_PASSWORD(chaimir_app 角色口令,与后端应用连接一致)}"

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"

echo "[1/1] 执行数据库迁移与应用角色授权..."
if command -v chaimir-migrate >/dev/null 2>&1; then
  (
    cd "$ROOT/backend"
    chaimir-migrate migrate
  )
elif command -v go >/dev/null 2>&1; then
  (
    cd "$ROOT/backend"
    go run ./cmd/migrate migrate
  )
else
  echo "未找到 chaimir-migrate 或 go,无法执行统一迁移入口。" >&2
  exit 1
fi

echo "建库初始化完成:迁移已应用,chaimir_app 角色就绪。"
