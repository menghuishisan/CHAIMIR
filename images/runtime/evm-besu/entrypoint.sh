#!/usr/bin/env sh
# 本脚本在私有 runtime-state 中生成单节点 QBFT 教学链并启动 Besu。
set -eu
: "${CHAIMIR_RUNTIME_RPC_PORT:?CHAIMIR_RUNTIME_RPC_PORT is required}"

case "$CHAIMIR_RUNTIME_RPC_PORT" in
  ''|*[!0-9]*) echo "CHAIMIR_RUNTIME_RPC_PORT must be a valid TCP port" >&2; exit 2 ;;
esac
if [ "$CHAIMIR_RUNTIME_RPC_PORT" -lt 1 ] || [ "$CHAIMIR_RUNTIME_RPC_PORT" -gt 65535 ]; then
  echo "CHAIMIR_RUNTIME_RPC_PORT must be in 1-65535" >&2
  exit 2
fi

state_root='/runtime-state/besu'
bootstrap_dir="$state_root/chaimir-qbft"
bootstrap_config='/opt/besu/chaimir-qbft-config.json'
genesis_file="$bootstrap_dir/genesis.json"

[ -s "$bootstrap_config" ] || { echo 'Besu QBFT bootstrap 配置缺失' >&2; exit 2; }
mkdir -p "$state_root"
if [ -e "$bootstrap_dir" ] && [ ! -d "$bootstrap_dir" ]; then
  echo 'Besu QBFT bootstrap 路径不是目录' >&2
  exit 2
fi
if [ ! -e "$bootstrap_dir" ]; then
  umask 077
  mkdir "$bootstrap_dir"
  /opt/besu/bin/besu operator generate-blockchain-config \
    --config-file="$bootstrap_config" \
    --to="$bootstrap_dir" \
    --private-key-file-name=key.priv \
    --public-key-file-name=key.pub
fi

[ -s "$genesis_file" ] || { echo 'Besu QBFT genesis 缺失或为空' >&2; exit 2; }
key_count="$(find "$bootstrap_dir/keys" -type f -name key.priv 2>/dev/null | wc -l | tr -d ' ')"
if [ "$key_count" -ne 1 ]; then
  echo 'Besu QBFT validator key 数量不正确' >&2
  exit 2
fi
node_key="$(find "$bootstrap_dir/keys" -type f -name key.priv -print)"
[ -s "$node_key" ] || { echo 'Besu QBFT validator key 缺失或为空' >&2; exit 2; }

# RPC 只监听容器端口,外部入口由 M2 控制面生成的 Service/Gateway 代理。
exec /opt/besu/bin/besu \
  --data-path="$state_root/data" \
  --genesis-file="$genesis_file" \
  --node-private-key-file="$node_key" \
  --rpc-http-enabled=true \
  --rpc-http-host=0.0.0.0 \
  --rpc-http-port="${CHAIMIR_RUNTIME_RPC_PORT}" \
  --rpc-http-api=ETH,NET,WEB3,TXPOOL,DEBUG \
  --host-allowlist='*'
