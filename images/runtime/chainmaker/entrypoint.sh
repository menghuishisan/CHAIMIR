#!/usr/bin/env sh
# 本脚本启动长安链节点或执行版本自检。
set -eu

if [ "${CHAIMIR_SELFTEST:-0}" = "1" ]; then
  exec chainmaker version
fi

# 长安链节点配置必须由 M2 注入,不能使用镜像内隐式配置。
: "${CHAIMIR_CHAINMAKER_CONFIG:?必须通过初始化卷提供长安链节点配置路径}"
exec chainmaker start -c "${CHAIMIR_CHAINMAKER_CONFIG}"
