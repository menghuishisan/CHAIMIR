#!/usr/bin/env sh
# 本脚本按运行期注入的 L1/L2/Rollup 配置启动 op-node,缺少配置时显式失败。
set -eu

: "${CHAIMIR_OP_NODE_L1_RPC:?必须提供 OP Stack L1 RPC 地址}"
: "${CHAIMIR_OP_NODE_L2_RPC:?必须提供 OP Stack L2 engine RPC 地址}"
: "${CHAIMIR_OP_NODE_ROLLUP_CONFIG:?必须提供 OP Stack rollup 配置文件路径}"

exec op-node \
  --l1="${CHAIMIR_OP_NODE_L1_RPC}" \
  --l2="${CHAIMIR_OP_NODE_L2_RPC}" \
  --rollup.config="${CHAIMIR_OP_NODE_ROLLUP_CONFIG}" \
  --rpc.addr=0.0.0.0 \
  --rpc.port="${CHAIMIR_OP_NODE_RPC_PORT:-8545}"
