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

# 使用平台统一注入的助记词派生运行时账户,并保留编排显式传入的参数。
exec anvil --host 0.0.0.0 --port "${CHAIMIR_RUNTIME_RPC_PORT}" --mnemonic "$CHAIMIR_EVM_DEPLOYER_MNEMONIC" "$@"
