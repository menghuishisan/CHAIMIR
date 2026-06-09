#!/usr/bin/env sh
# 本脚本启动仅限沙箱内部访问的 geth dev 链。
set -eu

# HTTP RPC 监听容器端口,真实对外入口由平台代理鉴权控制。
exec geth \
  --dev \
  --datadir /runtime-state/geth \
  --http \
  --http.addr 0.0.0.0 \
  --http.port "${CHAIMIR_RUNTIME_RPC_PORT:-8545}" \
  --http.api eth,net,web3,debug,txpool \
  --http.vhosts '*'
