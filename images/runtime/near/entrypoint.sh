#!/usr/bin/env sh
# 本脚本启动 NEAR 本地节点或执行版本自检。
set -eu

if [ "${CHAIMIR_SELFTEST:-0}" = "1" ]; then
  exec neard --version
fi

# 首次挂载空运行态卷时生成单节点 localnet 配置;已有节点配置不会被覆盖。
: "${CHAIMIR_NEAR_HOME:?必须通过初始化卷提供 NEAR home 目录}"
if [ ! -f "${CHAIMIR_NEAR_HOME}/node0/config.json" ]; then
  if ! neard --home "${CHAIMIR_NEAR_HOME}" localnet -v 1 >/dev/null 2>&1; then
    echo "生成 NEAR 本地链配置失败" >&2
    exit 1
  fi
fi
exec neard --home "${CHAIMIR_NEAR_HOME}/node0" run --rpc-addr 0.0.0.0:3030
