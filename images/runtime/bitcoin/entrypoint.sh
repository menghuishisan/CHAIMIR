#!/usr/bin/env sh
# 本脚本启动 Bitcoin regtest 节点。
set -eu

exec bitcoind \
  -regtest=1 \
  -datadir=/runtime-state/bitcoin \
  -server=1 \
  -rpcbind=0.0.0.0 \
  -rpcallowip=0.0.0.0/0 \
  -rpcport="${CHAIMIR_RUNTIME_RPC_PORT:-18443}" \
  -printtoconsole=1
