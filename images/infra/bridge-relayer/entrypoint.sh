#!/usr/bin/env sh
# 本脚本默认启动 Hyperlane relayer,并允许 M2 WorkloadSpec 显式传入受控命令。
set -eu

: "${HYP_BASE_CONFIG:?必须通过组合配置提供 Hyperlane base config 文件}"
: "${HYP_DB:?必须通过组合配置提供 Hyperlane 数据目录}"
: "${HYP_RELAYCHAINS:?必须通过组合配置提供 relayChains 配置}"
: "${HYP_SOURCE_RPC:?必须通过 source_chain binding 提供源链 RPC}"
: "${HYP_DESTINATION_RPC:?必须通过 destination_chain binding 提供目标链 RPC}"

if [ ! -f "$HYP_BASE_CONFIG" ]; then
  echo "Hyperlane base config 文件不存在: $HYP_BASE_CONFIG" >&2
  exit 2
fi
mkdir -p "$HYP_DB"
/usr/local/bin/chaimir-hyperlane-config
export CONFIG_FILES=/runtime-state/hyperlane/merged-config.json

if [ "$#" -eq 0 ]; then
  set -- /app/relayer
fi

cd /app
exec tini -- "$@"
