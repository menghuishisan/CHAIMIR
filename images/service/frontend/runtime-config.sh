#!/bin/sh
# 从统一非敏感环境变量生成浏览器运行时配置,拒绝未校验内容进入 JavaScript。
set -eu

case "${DEPLOY_MODE:-}" in
  saas|school) ;;
  *) echo "DEPLOY_MODE 必须为 saas 或 school" >&2; exit 1 ;;
esac

origin="${SANDBOX_TOOL_ORIGIN:-}"
case "$origin" in
  https://*) ;;
  *) echo "SANDBOX_TOOL_ORIGIN 必须为 HTTPS origin" >&2; exit 1 ;;
esac
host="${origin#https://}"
case "$host" in
  ""|*[!A-Za-z0-9._:-]*) echo "SANDBOX_TOOL_ORIGIN 只能包含主机名和端口" >&2; exit 1 ;;
esac

umask 022
printf "window.__CHAIMIR_RUNTIME_CONFIG__ = { deploymentMode: '%s', sandboxToolOrigin: '%s' };\n" \
  "$DEPLOY_MODE" "$origin" > /runtime-config/runtime-config.js
