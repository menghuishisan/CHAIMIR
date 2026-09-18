#!/usr/bin/env sh
# 本脚本启动仅限沙箱内部访问的 geth dev 链。
set -eu
: "${CHAIMIR_RUNTIME_RPC_PORT:?CHAIMIR_RUNTIME_RPC_PORT is required}"

case "$CHAIMIR_RUNTIME_RPC_PORT" in
  ''|*[!0-9]*) echo "CHAIMIR_RUNTIME_RPC_PORT must be a valid TCP port" >&2; exit 2 ;;
esac
if [ "$CHAIMIR_RUNTIME_RPC_PORT" -lt 1 ] || [ "$CHAIMIR_RUNTIME_RPC_PORT" -gt 65535 ]; then
  echo "CHAIMIR_RUNTIME_RPC_PORT must be in 1-65535" >&2
  exit 2
fi

if [ "$#" -gt 0 ]; then
  exec "$@"
fi

state_dir="/runtime-state/geth"
if [ -n "${CHAIMIR_EVM_DEPLOYER_PRIVATE_KEY:-}" ] && ! geth account list --datadir "$state_dir" 2>/dev/null | grep -q '^Account #'; then
  deployer_key="$CHAIMIR_EVM_DEPLOYER_PRIVATE_KEY"
  case "$deployer_key" in
    0x*) deployer_key="${deployer_key#0x}" ;;
    0X*) deployer_key="${deployer_key#0X}" ;;
  esac
  if [ "${#deployer_key}" -ne 64 ]; then
    echo "CHAIMIR_EVM_DEPLOYER_PRIVATE_KEY must be 64 hexadecimal characters" >&2
    exit 2
  fi
  case "$deployer_key" in
    *[!0-9a-fA-F]*)
      echo "CHAIMIR_EVM_DEPLOYER_PRIVATE_KEY must be 64 hexadecimal characters" >&2
      exit 2
      ;;
  esac
  mkdir -p "$state_dir/keystore"
  key_file="$state_dir/import.key"
  trap 'rm -f "$key_file"' EXIT HUP INT TERM
  printf '%s' "$deployer_key" > "$key_file"
  geth account import --datadir "$state_dir" --password /dev/null "$key_file" >/dev/null
  rm -f "$key_file"
  trap - EXIT HUP INT TERM
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
  --http.port "${CHAIMIR_RUNTIME_RPC_PORT}" \
  --http.api eth,net,web3,debug,txpool \
  --http.vhosts '*'
