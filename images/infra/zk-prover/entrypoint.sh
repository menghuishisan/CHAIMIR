#!/usr/bin/env sh
# 本脚本只执行 M2/runtime manifest 明确传入的证明命令,缺少命令时显式失败。
set -eu

if [ "$#" -eq 0 ]; then
  echo "zk prover command must be provided by runtime manifest" >&2
  exit 64
fi

exec "$@"
