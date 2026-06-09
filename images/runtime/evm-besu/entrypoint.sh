#!/usr/bin/env sh
# 本脚本启动仅限沙箱内部访问的 Besu 开发链。
set -eu

# RPC 只监听容器端口,外部入口由 M2 控制面生成的 Service/Gateway 代理。
exec besu \
  --data-path=/runtime-state/besu \
  --network=dev \
  --rpc-http-enabled=true \
  --rpc-http-host=0.0.0.0 \
  --rpc-http-port="${CHAIMIR_RUNTIME_RPC_PORT:-8545}" \
  --rpc-http-api=ETH,NET,WEB3,TXPOOL,DEBUG \
  --host-allowlist='*'
