#!/usr/bin/env sh
# 本脚本启动 Bitcoin regtest 节点。
set -eu
: "${CHAIMIR_BITCOIN_DATA_DIR:?CHAIMIR_BITCOIN_DATA_DIR is required}"
: "${CHAIMIR_RUNTIME_RPC_PORT:?CHAIMIR_RUNTIME_RPC_PORT is required}"
: "${CHAIMIR_BITCOIN_FALLBACK_FEE:?CHAIMIR_BITCOIN_FALLBACK_FEE is required}"

case "$CHAIMIR_RUNTIME_RPC_PORT" in
  ''|*[!0-9]*) echo "CHAIMIR_RUNTIME_RPC_PORT must be a valid TCP port" >&2; exit 2 ;;
esac
if [ "$CHAIMIR_RUNTIME_RPC_PORT" -lt 1 ] || [ "$CHAIMIR_RUNTIME_RPC_PORT" -gt 65535 ]; then
  echo "CHAIMIR_RUNTIME_RPC_PORT must be in 1-65535" >&2
  exit 2
fi

DATA_DIR="${CHAIMIR_BITCOIN_DATA_DIR}"
mkdir -p "${DATA_DIR}"

exec bitcoind \
  -regtest=1 \
  -datadir="${DATA_DIR}" \
  -server=1 \
  -rpcbind=0.0.0.0 \
  -rpcallowip=0.0.0.0/0 \
  -rpcport="${CHAIMIR_RUNTIME_RPC_PORT}" \
  -fallbackfee="${CHAIMIR_BITCOIN_FALLBACK_FEE}" \
  -printtoconsole=1
