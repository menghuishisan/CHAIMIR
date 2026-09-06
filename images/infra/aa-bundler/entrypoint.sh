#!/usr/bin/env sh
# 本脚本按运行期注入的配置启动 ERC-4337 bundler,缺少配置时显式失败。
set -eu

: "${NODE_HTTP:?必须通过运行时配置提供 EVM 节点 HTTP RPC 地址}"
: "${SIGNER_PRIVATE_KEYS:?必须通过受控 Secret 提供 bundler signer 私钥}"

exec rundler node "$@"
