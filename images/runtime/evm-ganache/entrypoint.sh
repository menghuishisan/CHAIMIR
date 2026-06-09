#!/usr/bin/env sh
# 本脚本启动 Ganache 轻量测试链,用于快速教学实验。
set -eu

# Ganache 数据写入 runtime-state,容器外部入口由平台统一控制。
exec ganache \
  --host 0.0.0.0 \
  --port "${CHAIMIR_RUNTIME_RPC_PORT:-8545}" \
  --database.dbPath /runtime-state/ganache
