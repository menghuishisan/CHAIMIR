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
case "$BEACON_EXECUTION_ENDPOINT" in
  http://*|https://*) ;;
  *) echo "BEACON_EXECUTION_ENDPOINT 必须是 HTTP(S) 地址" >&2; exit 2 ;;
esac
execution_target="${BEACON_EXECUTION_ENDPOINT#*://}"
case "$execution_target" in
  *@*|*/*|*\?*|*\#*) echo "BEACON_EXECUTION_ENDPOINT 不得包含凭据、路径、查询或片段" >&2; exit 2 ;;
esac
execution_host="${execution_target%:*}"
execution_port="${execution_target##*:}"
case "$execution_host" in
  ''|*[!A-Za-z0-9.-]*) echo "BEACON_EXECUTION_ENDPOINT host 无效" >&2; exit 2 ;;
esac
if [ "$execution_host" = "$execution_target" ] || [ -z "$execution_port" ]; then
  echo "BEACON_EXECUTION_ENDPOINT 必须显式包含 host 和端口" >&2
  exit 2
fi
case "$execution_port" in
  ''|*[!0-9]*) echo "BEACON_EXECUTION_ENDPOINT 端口无效" >&2; exit 2 ;;
esac
if [ "$execution_port" -lt 1 ] || [ "$execution_port" -gt 65535 ]; then
  echo "BEACON_EXECUTION_ENDPOINT 端口必须位于 1-65535" >&2
  exit 2
fi
case "$BEACON_CHECKPOINT_SYNC_URL" in
  http://*|https://*) ;;
  *)
    echo "BEACON_CHECKPOINT_SYNC_URL 必须是 http(s) 地址" >&2
    exit 2
    ;;
esac
checkpoint_target="${BEACON_CHECKPOINT_SYNC_URL#*://}"
case "$checkpoint_target" in
  *@*|*\?*|*\#*) echo "BEACON_CHECKPOINT_SYNC_URL 不得包含凭据、查询或片段" >&2; exit 2 ;;
esac
checkpoint_authority="${checkpoint_target%%/*}"
if [ "$checkpoint_authority" = "${checkpoint_authority#*:}" ]; then
  checkpoint_host="$checkpoint_authority"
  checkpoint_port=""
else
  checkpoint_host="${checkpoint_authority%:*}"
  checkpoint_port="${checkpoint_authority##*:}"
fi
case "$checkpoint_host" in
  ''|*[!A-Za-z0-9.-]*) echo "BEACON_CHECKPOINT_SYNC_URL host 无效" >&2; exit 2 ;;
esac
if [ -n "$checkpoint_port" ]; then
  case "$checkpoint_port" in
    *[!0-9]*) echo "BEACON_CHECKPOINT_SYNC_URL 端口无效" >&2; exit 2 ;;
  esac
  if [ "$checkpoint_port" -lt 1 ] || [ "$checkpoint_port" -gt 65535 ]; then
    echo "BEACON_CHECKPOINT_SYNC_URL 端口必须位于 1-65535" >&2
    exit 2
  fi
fi
case "$BEACON_HTTP_PORT" in
  *[!0-9]*|'')
    echo "BEACON_HTTP_PORT 必须是数字端口" >&2
    exit 2
    ;;
esac
if [ "$BEACON_HTTP_PORT" -lt 1 ] || [ "$BEACON_HTTP_PORT" -gt 65535 ]; then
  echo "BEACON_HTTP_PORT 必须位于 1-65535" >&2
  exit 2
fi

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
