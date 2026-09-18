#!/usr/bin/env sh
# 本脚本启动 Aptos 受控教学测试网。
set -eu

export HOME=/runtime-state/aptos
export TMPDIR=/runtime-state/aptos
mkdir -p "$HOME"
exec aptos node run-local-testnet \
  --test-dir /runtime-state/aptos \
  --with-faucet \
  --force-restart \
  --assume-yes \
  --bind-to 0.0.0.0
