#!/usr/bin/env sh
# 本脚本启动 Sui 本地网络。
set -eu

CONFIG_DIR="${CHAIMIR_SUI_CONFIG_DIR:-/runtime-state/sui}"
if [ ! -f "$CONFIG_DIR/network.yaml" ]; then
  /usr/local/bin/sui genesis --working-dir "$CONFIG_DIR" --with-faucet -f
fi
exec /usr/local/bin/sui start --with-faucet --network.config "$CONFIG_DIR"
