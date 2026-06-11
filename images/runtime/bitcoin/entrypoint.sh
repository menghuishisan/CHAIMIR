#!/usr/bin/env sh
# 本脚本启动 Bitcoin regtest 节点。
set -eu

DATA_DIR="${CHAIMIR_BITCOIN_DATA_DIR:-/runtime-state/bitcoin}"
mkdir -p "${DATA_DIR}"

exec bitcoind \
  -regtest=1 \
  -datadir="${DATA_DIR}" \
  -server=1 \
  -rpcbind=0.0.0.0 \
  -rpcallowip=0.0.0.0/0 \
  -rpcport="${CHAIMIR_RUNTIME_RPC_PORT:-18443}" \
  -printtoconsole=1
