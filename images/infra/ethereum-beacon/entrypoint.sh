#!/usr/bin/env sh
# 本脚本把组合 binding、Secret 和受控共识材料转换为 Lighthouse beacon_node argv。
set -eu

: "${BEACON_EXECUTION_ENDPOINT:?必须通过 execution_chain binding 提供执行层端点}"
: "${BEACON_EXECUTION_JWT:?必须通过受控 Secret 提供执行层 JWT}"
: "${BEACON_DATA_DIR:?必须通过组合配置提供 Beacon 数据目录}"
: "${BEACON_TESTNET_DIR:?必须通过受控初始化材料提供 Beacon testnet 目录}"
: "${BEACON_CHECKPOINT_STATE_PATH:?必须通过受控初始化材料提供 Beacon checkpoint state}"
: "${BEACON_CHECKPOINT_BLOCK_PATH:?必须通过受控初始化材料提供 Beacon checkpoint block}"
: "${BEACON_HTTP_PORT:?必须通过组合配置提供 Beacon HTTP 端口}"

case "$BEACON_DATA_DIR" in
  /runtime-state/*|/runtime-state) ;;
  *)
    echo "BEACON_DATA_DIR 必须位于 runtime-state 安全域" >&2
    exit 2
    ;;
esac
for material_path in "$BEACON_TESTNET_DIR" "$BEACON_CHECKPOINT_STATE_PATH" "$BEACON_CHECKPOINT_BLOCK_PATH"; do
  case "$material_path" in
    /runtime-state/*|/runtime-state) ;;
    *) echo "Beacon 受控材料必须位于 runtime-state 安全域" >&2; exit 2 ;;
  esac
done
if [ ! -d "$BEACON_TESTNET_DIR" ]; then
  echo "BEACON_TESTNET_DIR 必须指向已注入的受控 testnet 目录" >&2
  exit 2
fi
for material_file in "$BEACON_TESTNET_DIR/config.yaml" "$BEACON_TESTNET_DIR/genesis.ssz" "$BEACON_CHECKPOINT_STATE_PATH" "$BEACON_CHECKPOINT_BLOCK_PATH"; do
  if [ ! -s "$material_file" ]; then
    echo "Beacon 受控材料缺失或为空: $material_file" >&2
    exit 2
  fi
done
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
  --testnet-dir "$BEACON_TESTNET_DIR" \
  --datadir "$BEACON_DATA_DIR" \
  --execution-endpoint "$BEACON_EXECUTION_ENDPOINT" \
  --execution-jwt "$jwt_path" \
  --checkpoint-state "$BEACON_CHECKPOINT_STATE_PATH" \
  --checkpoint-block "$BEACON_CHECKPOINT_BLOCK_PATH" \
  --http \
  --http-address 0.0.0.0 \
  --http-port "$BEACON_HTTP_PORT" \
  "$@"
