#!/usr/bin/env sh
# 本脚本默认启动 Hyperlane relayer,并允许 M2 WorkloadSpec 显式传入受控命令。
set -eu

export CHAIMIR_BRIDGE_WORKSPACE="${CHAIMIR_BRIDGE_WORKSPACE:-/workspace}"

if [ "$#" -eq 0 ]; then
  set -- /app/relayer
fi

cd /app
exec "$@"
