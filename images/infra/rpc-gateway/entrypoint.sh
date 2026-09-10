#!/usr/bin/env sh
# 本脚本将组合绑定的 EVM JSON-RPC URL 渲染为 Envoy 受控配置并启动网关。
set -eu

: "${CHAIMIR_RPC_UPSTREAM_URL:?必须通过组合绑定提供 EVM JSON-RPC 地址}"
case "$CHAIMIR_RPC_UPSTREAM_URL" in
  http://*|https://*) ;;
  *) echo "CHAIMIR_RPC_UPSTREAM_URL 必须使用 HTTP(S) 协议" >&2; exit 2 ;;
esac
target="${CHAIMIR_RPC_UPSTREAM_URL#*://}"
case "$target" in
  *@*|*/*|*\?*|*\#*)
    echo "CHAIMIR_RPC_UPSTREAM_URL 不得包含凭据、路径、查询或片段" >&2
    exit 2
    ;;
esac
host="${target%:*}"
port="${target##*:}"
case "$host" in
  ''|*[!A-Za-z0-9.-]*) echo "CHAIMIR_RPC_UPSTREAM_URL host 无效" >&2; exit 2 ;;
esac
if [ "$host" = "$target" ] || [ -z "$port" ]; then
  echo "CHAIMIR_RPC_UPSTREAM_URL 必须显式包含端口" >&2
  exit 2
fi
case "$port" in
  ''|*[!0-9]*) echo "CHAIMIR_RPC_UPSTREAM_URL 端口无效" >&2; exit 2 ;;
esac
if [ "$port" -lt 1 ] || [ "$port" -gt 65535 ]; then
  echo "CHAIMIR_RPC_UPSTREAM_URL 端口超出范围" >&2
  exit 2
fi

rendered="/tmp/chaimir-envoy.yaml"
sed -e "s|__UPSTREAM_HOST__|$host|g" -e "s|__UPSTREAM_PORT__|$port|g" \
  /etc/chaimir/envoy.yaml > "$rendered"
exec envoy -c "$rendered" "$@"
