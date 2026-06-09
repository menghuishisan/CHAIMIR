#!/usr/bin/env sh
# 本脚本启动 Aptos 本地测试网。
set -eu

exec aptos node run-local-testnet \
  --test-dir /runtime-state/aptos \
  --with-faucet \
  --force-restart
