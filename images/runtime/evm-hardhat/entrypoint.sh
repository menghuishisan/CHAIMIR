#!/usr/bin/env sh
# 本脚本启动 Hardhat 运行时默认链节点。
set -eu

# 使用 Anvil 提供稳定 JSON-RPC,Hardhat 工具链留给学生在工作区内使用。
exec anvil --host 0.0.0.0 --port "${CHAIMIR_RUNTIME_RPC_PORT:-8545}"
