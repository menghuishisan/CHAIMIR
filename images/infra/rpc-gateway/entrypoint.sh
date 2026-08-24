#!/usr/bin/env sh
# 本脚本将组合绑定的 EVM JSON-RPC URL 渲染为 Envoy 受控配置并启动网关。
set -eu

: "${CHAIMIR_RPC_UPSTREAM_URL:?必须通过组合绑定提供 EVM JSON-RPC 地址}"
target="${CHAIMIR_RPC_UPSTREAM_URL#*://}"
target="${target##*@}"
target="${target%%/*}"
target="${target%%\?*}"
host="${target%:*}"
port="${target##*:}"
if [ "$host" = "$target" ]; then
  port=8545
fi
case "$port" in
  ''|*[!0-9]*) echo "CHAIMIR_RPC_UPSTREAM_URL 端口无效" >&2; exit 2 ;;
esac

rendered="/tmp/chaimir-envoy.yaml"
sed -e "s|__UPSTREAM_HOST__|$host|g" -e "s|__UPSTREAM_PORT__|$port|g" \
  /etc/chaimir/envoy.yaml > "$rendered"
exec envoy -c "$rendered" "$@"
