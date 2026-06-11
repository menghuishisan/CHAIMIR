#!/usr/bin/env sh
# 本脚本只执行 M2/runtime manifest 明确传入的中继命令,缺少命令时显式失败。
set -eu

export NODE_PATH="${NODE_PATH:-/opt/chaimir/bridge-relayer/node_modules}"
export CHAIMIR_BRIDGE_WORKSPACE="${CHAIMIR_BRIDGE_WORKSPACE:-/workspace}"

if [ "$#" -eq 0 ]; then
  echo "bridge relayer command must be provided by runtime manifest" >&2
  exit 64
fi

cd /opt/chaimir/bridge-relayer
exec "$@"
