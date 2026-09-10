#!/usr/bin/env sh
# 本脚本启动 Substrate 开发链。
set -eu
: "${CHAIMIR_RUNTIME_RPC_PORT:?CHAIMIR_RUNTIME_RPC_PORT is required}"

case "$CHAIMIR_RUNTIME_RPC_PORT" in
  ''|*[!0-9]*) echo "CHAIMIR_RUNTIME_RPC_PORT must be a valid TCP port" >&2; exit 2 ;;
esac
if [ "$CHAIMIR_RUNTIME_RPC_PORT" -lt 1 ] || [ "$CHAIMIR_RUNTIME_RPC_PORT" -gt 65535 ]; then
  echo "CHAIMIR_RUNTIME_RPC_PORT must be in 1-65535" >&2
  exit 2
fi

exec polkadot \
  --dev \
  --base-path /runtime-state/substrate \
  --unsafe-rpc-external \
  --rpc-port "${CHAIMIR_RUNTIME_RPC_PORT}" \
  --rpc-cors all
