#!/usr/bin/env sh
# 本脚本启动 FISCO BCOS 节点或执行版本自检。
set -eu

if [ "${CHAIMIR_SELFTEST:-0}" = "1" ]; then
  exec fisco-bcos --version
fi

# 节点配置由 M2 初始化卷注入,缺失时显式失败,避免启动不受控默认网络。
: "${CHAIMIR_FISCO_CONFIG:?必须通过初始化卷提供 FISCO BCOS 节点配置路径}"
exec fisco-bcos -c "${CHAIMIR_FISCO_CONFIG}"
