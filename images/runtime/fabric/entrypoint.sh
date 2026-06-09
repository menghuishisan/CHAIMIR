#!/usr/bin/env bash
# 本脚本提供 Fabric 工具容器入口,具体 peer/orderer/CA 组网由 M2 编排。
set -euo pipefail

# 默认输出版本用于自检;实际实验命令由平台通过参数覆盖。
if [[ "$#" -eq 0 ]]; then
  peer version
  exit 0
fi

exec "$@"
