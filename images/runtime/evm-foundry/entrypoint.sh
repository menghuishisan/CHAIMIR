#!/usr/bin/env sh
# 本脚本启动 Anvil 测试链,用于 Chaimir EVM 运行时容器。
set -eu
: "${CHAIMIR_CHAIN_STATE_DIR:?CHAIMIR_CHAIN_STATE_DIR is required}"
: "${CHAIMIR_RUNTIME_RPC_PORT:?CHAIMIR_RUNTIME_RPC_PORT is required}"
: "${CHAIMIR_EVM_DEPLOYER_MNEMONIC:?CHAIMIR_EVM_DEPLOYER_MNEMONIC is required}"

mnemonic_words=$(printf '%s' "$CHAIMIR_EVM_DEPLOYER_MNEMONIC" | awk '{print NF}')
case "$mnemonic_words" in
  12|15|18|21|24) ;;
  *) echo "CHAIMIR_EVM_DEPLOYER_MNEMONIC 必须是有效长度的 BIP39 助记词" >&2; exit 2 ;;
esac

case "$CHAIMIR_RUNTIME_RPC_PORT" in
  ''|*[!0-9]*) echo "CHAIMIR_RUNTIME_RPC_PORT must be a valid TCP port" >&2; exit 2 ;;
esac
if [ "$CHAIMIR_RUNTIME_RPC_PORT" -lt 1 ] || [ "$CHAIMIR_RUNTIME_RPC_PORT" -gt 65535 ]; then
  echo "CHAIMIR_RUNTIME_RPC_PORT must be in 1-65535" >&2
  exit 2
fi

# 将镜像内固定的多版本编译器复制到运行态 SVM 目录,避免把编译器缓存写入学生 workspace。
seed_solidity_compilers() {
  svm_root="${HOME}/.svm"
  for version in 0.8.15 0.8.19 0.8.20 0.8.24 0.8.25 0.8.26; do
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

# 任一 fork 身份项出现时必须完整握手；不允许载入普通链状态或用附加 CLI 改写身份。
if [ "${CHAIMIR_EVM_FORK_RPC_URL+x}${CHAIMIR_EVM_FORK_BLOCK_NUMBER+x}${CHAIMIR_EVM_FORK_BLOCK_HASH+x}${CHAIMIR_EVM_FORK_STATE_ROOT+x}${CHAIMIR_EVM_FORK_CHAIN_ID+x}" != "" ]; then
  : "${CHAIMIR_EVM_FORK_RPC_URL:?历史状态私有连接缺失}"
  : "${CHAIMIR_EVM_FORK_BLOCK_NUMBER:?固定历史区块号缺失}"
  : "${CHAIMIR_EVM_FORK_BLOCK_HASH:?固定历史区块哈希缺失}"
  : "${CHAIMIR_EVM_FORK_STATE_ROOT:?固定历史状态根缺失}"
  : "${CHAIMIR_EVM_FORK_CHAIN_ID:?固定历史链编号缺失}"
  [ "$#" -eq 0 ] || { echo "历史 fork 不接受附加启动参数" >&2; exit 2; }
  case "$CHAIMIR_EVM_FORK_RPC_URL" in http://*) ;; *) echo "历史状态必须来自沙箱私有 HTTP 连接" >&2; exit 2 ;; esac
  block_hex=$(cast to-hex "$CHAIMIR_EVM_FORK_BLOCK_NUMBER")
  chain_hex=$(cast to-hex "$CHAIMIR_EVM_FORK_CHAIN_ID")
  chain_response=$(cast rpc --rpc-url "$CHAIMIR_EVM_FORK_RPC_URL" --rpc-timeout 10 eth_chainId)
  printf '%s' "$chain_response" | jq -e -s --arg expected "$chain_hex" '. == [$expected]' >/dev/null
  block_response=$(cast rpc --rpc-url "$CHAIMIR_EVM_FORK_RPC_URL" --rpc-timeout 10 eth_getBlockByNumber "$block_hex" false)
  printf '%s' "$block_response" |
    jq -e -s --arg number "$block_hex" --arg hash "$CHAIMIR_EVM_FORK_BLOCK_HASH" --arg root "$CHAIMIR_EVM_FORK_STATE_ROOT" \
      'length == 1 and .[0].number == $number and .[0].hash == $hash and .[0].stateRoot == $root' >/dev/null
  exec anvil --quiet --host 0.0.0.0 --port "$CHAIMIR_RUNTIME_RPC_PORT" \
    --fork-url "$CHAIMIR_EVM_FORK_RPC_URL" --fork-block-number "$CHAIMIR_EVM_FORK_BLOCK_NUMBER" \
    --chain-id "$CHAIMIR_EVM_FORK_CHAIN_ID" --no-storage-caching --mnemonic "$CHAIMIR_EVM_DEPLOYER_MNEMONIC"
fi

# 只监听容器网络地址,外部访问必须由平台 Service/Gateway 控制。
# 使用镜像内固化的初始状态,启动时复制到可写运行态目录,避免修改只读镜像层。
state_root="${CHAIMIR_CHAIN_STATE_DIR}/foundry"
state_file="$state_root/genesis-state.json"
mkdir -p "$state_root"
if [ ! -s "$state_file" ]; then
  cp /usr/local/share/chaimir/foundry/genesis-state.json "$state_file"
fi

# Anvil 默认会打印测试账户私钥和助记词,生产日志必须静默启动。
# 组合显式传入的参数属于运行时契约,例如双链联动需要独立的 chain id。
exec anvil --quiet --host 0.0.0.0 --port "${CHAIMIR_RUNTIME_RPC_PORT}" --state "$state_file" --mnemonic "$CHAIMIR_EVM_DEPLOYER_MNEMONIC" "$@"
