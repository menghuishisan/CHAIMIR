#!/usr/bin/env sh
# 本脚本按运行期注入的统一配置启动 ERC-4337 bundler,缺少链规范或密钥时显式失败。
set -eu

: "${NODE_HTTP:?必须通过运行时配置提供 EVM 节点 HTTP RPC 地址}"
: "${SIGNER_PRIVATE_KEYS:?必须通过受控 Secret 提供 bundler signer 私钥}"
: "${CHAIN_SPEC:?必须通过受控初始化材料提供 Rundler 链规范文件}"

case "$CHAIN_SPEC" in
  /*) ;;
  *) echo "CHAIN_SPEC 必须是绝对路径" >&2; exit 2 ;;
esac
if [ ! -r "$CHAIN_SPEC" ]; then
  echo "CHAIN_SPEC 文件不存在或不可读" >&2
  exit 2
fi

case "$NODE_HTTP" in
  http://*|https://*) ;;
  *) echo "NODE_HTTP 必须使用 HTTP(S) 协议" >&2; exit 2 ;;
esac
target="${NODE_HTTP#*://}"
target="${target%%/*}"
target="${target%%\?*}"
target="${target%%\#*}"
case "$target" in
  ''|*@*|*:*:*) echo "NODE_HTTP 必须是无凭据的 host:port 地址" >&2; exit 2 ;;
esac
host="${target%:*}"
port="${target##*:}"
if [ "$host" = "$target" ] || [ -z "$host" ] || [ -z "$port" ]; then
  echo "NODE_HTTP 必须包含显式 host 和端口" >&2
  exit 2
fi
case "$port" in
  ''|*[!0-9]*) echo "NODE_HTTP 端口无效" >&2; exit 2 ;;
esac
if [ "$port" -lt 1 ] || [ "$port" -gt 65535 ]; then
  echo "NODE_HTTP 端口必须位于 1-65535" >&2
  exit 2
fi

exec rundler node "$@"
