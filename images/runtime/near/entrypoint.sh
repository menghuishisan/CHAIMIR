#!/usr/bin/env sh
# 本脚本启动 NEAR 本地节点或执行版本自检。
set -eu

if [ "${CHAIMIR_SELFTEST:-0}" = "1" ]; then
  exec neard --version
fi

# NEAR home 必须由 M2 初始化,避免隐式生成不受控网络。
: "${CHAIMIR_NEAR_HOME:?必须通过初始化卷提供 NEAR home 目录}"
exec neard run --home "${CHAIMIR_NEAR_HOME}"
