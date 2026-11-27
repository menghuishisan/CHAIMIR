#!/usr/bin/env sh
# 本脚本启动 Hardhat 运行时默认链节点。
set -eu
: "${CHAIMIR_RUNTIME_RPC_PORT:?CHAIMIR_RUNTIME_RPC_PORT is required}"

case "$CHAIMIR_RUNTIME_RPC_PORT" in
  ''|*[!0-9]*) echo "CHAIMIR_RUNTIME_RPC_PORT must be a valid TCP port" >&2; exit 2 ;;
esac
if [ "$CHAIMIR_RUNTIME_RPC_PORT" -lt 1 ] || [ "$CHAIMIR_RUNTIME_RPC_PORT" -gt 65535 ]; then
  echo "CHAIMIR_RUNTIME_RPC_PORT must be in 1-65535" >&2
  exit 2
fi

# 将镜像内固定的多版本编译器复制到运行态 SVM 目录,避免把编译器缓存写入工作区。
seed_solidity_compilers() {
  /usr/local/bin/node -e '
const fs = require("fs");
const path = require("path");
for (const version of ["0.8.20", "0.8.24", "0.8.26"]) {
  const compiler = `/usr/local/bin/solc-${version}`;
  if (!fs.existsSync(compiler)) throw new Error(`缺少内置 Solidity 编译器 ${version}`);
  const target = path.join(process.env.HOME, ".svm", version, `solc-${version}`);
  if (!fs.existsSync(target)) {
    fs.mkdirSync(path.dirname(target), { recursive: true });
    fs.copyFileSync(compiler, target);
    fs.chmodSync(target, 0o755);
  }
}
'
}

seed_solidity_compilers

: "${CHAIMIR_EVM_DEPLOYER_MNEMONIC:?CHAIMIR_EVM_DEPLOYER_MNEMONIC is required}"
mnemonic_words=0
for word in $CHAIMIR_EVM_DEPLOYER_MNEMONIC; do
  mnemonic_words=$((mnemonic_words + 1))
done
case "$mnemonic_words" in
  12|15|18|21|24) ;;
  *) echo "CHAIMIR_EVM_DEPLOYER_MNEMONIC 必须是有效长度的 BIP39 助记词" >&2; exit 2 ;;
esac

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

# 从平台助记词派生账户；关闭默认的私钥和助记词输出，保留编排显式参数。
exec anvil --quiet --host 0.0.0.0 --port "${CHAIMIR_RUNTIME_RPC_PORT}" --mnemonic "$CHAIMIR_EVM_DEPLOYER_MNEMONIC" "$@"
