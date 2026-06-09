#!/usr/bin/env sh
# 本脚本启动 Cosmos SDK 单链或执行版本自检。
set -eu

if [ "${CHAIMIR_SELFTEST:-0}" = "1" ]; then
  exec gaiad version
fi

# 创世文件和节点配置必须由 M2 初始化卷提供。
: "${CHAIMIR_COSMOS_HOME:?必须通过初始化卷提供 Cosmos home 目录}"
exec gaiad start --home "${CHAIMIR_COSMOS_HOME}"
