#!/usr/bin/env sh
# 本脚本启动 Hardhat 运行时默认链节点。
set -eu

# 将镜像内固定的多版本编译器复制到运行态 SVM 目录,避免把编译器缓存写入学生 workspace。
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

# 使用 Anvil 提供稳定 JSON-RPC,Hardhat 工具链留给学生在工作区内使用。
exec anvil --host 0.0.0.0 --port "${CHAIMIR_RUNTIME_RPC_PORT:-8545}"
