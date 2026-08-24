#!/usr/bin/env sh
# 本脚本启动 Solana test-validator,用于本地链教学实验。
set -eu

LEDGER_DIR="${CHAIMIR_SOLANA_LEDGER_DIR:-/runtime-state/solana/ledger}"
if [ ! -d "${LEDGER_DIR}" ]; then
  echo "Solana ledger directory does not exist: ${LEDGER_DIR}" >&2
  exit 1
fi

set -- --ledger "${LEDGER_DIR}" \
  --rpc-port "${CHAIMIR_RUNTIME_RPC_PORT:-8899}" \
  --bind-address 0.0.0.0

# A fresh persistent volume has no genesis; reset is required only for that first boot.
if [ ! -f "${LEDGER_DIR}/genesis.bin" ]; then
  set -- "$@" --reset
fi

# 只监听容器网络,账本目录由 runtime-state 卷承载。
exec solana-test-validator "$@"
