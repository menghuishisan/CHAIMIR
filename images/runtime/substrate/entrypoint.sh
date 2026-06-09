#!/usr/bin/env sh
# 本脚本启动 Substrate 开发链。
set -eu

exec substrate \
  --dev \
  --base-path /runtime-state/substrate \
  --unsafe-rpc-external \
  --rpc-port "${CHAIMIR_RUNTIME_RPC_PORT:-9944}"
