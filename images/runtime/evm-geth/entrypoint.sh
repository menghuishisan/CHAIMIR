#!/usr/bin/env sh
# 本脚本启动仅限沙箱内部访问的 geth dev 链。
set -eu

if [ "$#" -gt 0 ]; then
  exec "$@"
fi

state_dir="/runtime-state/geth"
if [ -n "${CHAIMIR_EVM_DEPLOYER_PRIVATE_KEY:-}" ] && ! geth account list --datadir "$state_dir" 2>/dev/null | grep -q '^Account #'; then
  mkdir -p "$state_dir/keystore"
  printf '%s\n' "$CHAIMIR_EVM_DEPLOYER_PRIVATE_KEY" > "$state_dir/import.key"
  geth account import --datadir "$state_dir" --password /dev/null "$state_dir/import.key" >/dev/null
  rm -f "$state_dir/import.key"
fi

# HTTP RPC 监听容器端口,真实对外入口由平台代理鉴权控制。
# Kubernetes 为名为 geth 的 Service 注入 GETH_* 变量,这些变量会被
# go-ethereum 解释为 CLI 环境配置;清除它们避免 Service 地址污染节点参数。
exec env \
  -u GETH_SERVICE_HOST \
  -u GETH_SERVICE_PORT \
  -u GETH_PORT \
  -u GETH_PORT_8545_TCP \
  -u GETH_PORT_8545_TCP_ADDR \
  -u GETH_PORT_8545_TCP_PORT \
  -u GETH_PORT_8545_TCP_PROTO \
  geth \
  --dev \
  --datadir "$state_dir" \
  --http \
  --http.addr 0.0.0.0 \
  --http.port "${CHAIMIR_RUNTIME_RPC_PORT:-8545}" \
  --http.api eth,net,web3,debug,txpool \
  --http.vhosts '*'
