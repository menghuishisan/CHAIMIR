#!/usr/bin/env sh
# 本脚本启动 Sui 本地网络。
set -eu

exec sui start \
  --with-faucet \
  --force-regenesis \
  --network.config /runtime-state/sui/network.yaml
