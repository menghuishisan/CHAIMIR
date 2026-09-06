#!/usr/bin/env sh
# 本脚本启动 Anvil 测试链,用于 Chaimir EVM 运行时容器。
set -eu

# 将镜像内固定的多版本编译器复制到运行态 SVM 目录,避免把编译器缓存写入学生 workspace。
seed_solidity_compilers() {
  svm_root="${HOME}/.svm"
  for version in 0.8.20 0.8.24 0.8.26; do
    compiler="/usr/local/bin/solc-${version}"
    target="${svm_root}/${version}/solc-${version}"
    [ -x "$compiler" ] || { echo "缺少内置 Solidity 编译器 ${version}" >&2; exit 70; }
    if [ ! -x "$target" ]; then
      mkdir -p "$(dirname "$target")"
      cp "$compiler" "$target"
      chmod 0755 "$target"
    fi
  done
}

seed_solidity_compilers

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
