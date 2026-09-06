#!/usr/bin/env sh
# 本脚本把组合 binding、Secret 和显式网络参数转换为 Lighthouse beacon_node argv。
set -eu

: "${BEACON_EXECUTION_ENDPOINT:?必须通过 execution_chain binding 提供执行层端点}"
: "${BEACON_EXECUTION_JWT:?必须通过受控 Secret 提供执行层 JWT}"
: "${BEACON_NETWORK:?必须通过组合配置提供 Beacon 网络}"
: "${BEACON_DATA_DIR:?必须通过组合配置提供 Beacon 数据目录}"
: "${BEACON_CHECKPOINT_SYNC_URL:?必须通过受控组合材料提供 Beacon checkpoint sync 地址}"
: "${BEACON_HTTP_PORT:?必须通过组合配置提供 Beacon HTTP 端口}"

case "$BEACON_DATA_DIR" in
  /runtime-state/*|/runtime-state) ;;
  *)
    echo "BEACON_DATA_DIR 必须位于 runtime-state 安全域" >&2
    exit 2
    ;;
esac
case "$BEACON_NETWORK" in
  mainnet|gnosis|chiado|sepolia|holesky|hoodi) ;;
  *)
    echo "BEACON_NETWORK 不是 Lighthouse 支持的网络" >&2
    exit 2
    ;;
esac
case "$BEACON_CHECKPOINT_SYNC_URL" in
  http://*|https://*) ;;
  *)
    echo "BEACON_CHECKPOINT_SYNC_URL 必须是 http(s) 地址" >&2
    exit 2
    ;;
esac
case "$BEACON_HTTP_PORT" in
  *[!0-9]*|'')
    echo "BEACON_HTTP_PORT 必须是数字端口" >&2
    exit 2
    ;;
esac

case "$BEACON_EXECUTION_JWT" in
  *[!0123456789abcdefABCDEF]*)
    echo "BEACON_EXECUTION_JWT 必须是 32 字节十六进制值" >&2
    exit 2
    ;;
esac
if [ "${#BEACON_EXECUTION_JWT}" -ne 64 ]; then
  echo "BEACON_EXECUTION_JWT 必须是 32 字节十六进制值" >&2
  exit 2
fi

runtime_root=/runtime-state/lighthouse
jwt_path="$runtime_root/execution.jwt"
mkdir -p "$runtime_root" "$BEACON_DATA_DIR"
umask 077
printf '%s\n' "$BEACON_EXECUTION_JWT" > "$jwt_path"

exec lighthouse beacon_node \
  --network "$BEACON_NETWORK" \
  --datadir "$BEACON_DATA_DIR" \
  --execution-endpoint "$BEACON_EXECUTION_ENDPOINT" \
  --execution-jwt "$jwt_path" \
  --checkpoint-sync-url "$BEACON_CHECKPOINT_SYNC_URL" \
  --http \
  --http-address 0.0.0.0 \
  --http-port "$BEACON_HTTP_PORT" \
  "$@"
