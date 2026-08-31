#!/usr/bin/env sh
# 本脚本启动 Anvil 测试链,用于 Chaimir EVM 运行时容器。
set -eu

# 只监听容器网络地址,外部访问必须由平台 Service/Gateway 控制。
# 使用镜像内固化的初始状态,启动时复制到可写运行态目录,避免修改只读镜像层。
state_root="${CHAIMIR_CHAIN_STATE_DIR:-/runtime-state}/foundry"
state_file="$state_root/genesis-state.json"
mkdir -p "$state_root"
if [ ! -s "$state_file" ]; then
  cp /usr/local/share/chaimir/foundry/genesis-state.json "$state_file"
fi

# Anvil 默认会打印测试账户私钥和助记词,生产日志必须静默启动。
exec anvil --quiet --host 0.0.0.0 --port "${CHAIMIR_RUNTIME_RPC_PORT:-8545}" --state "$state_file"
