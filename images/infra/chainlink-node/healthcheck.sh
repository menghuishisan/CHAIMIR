#!/usr/bin/env sh
# 本脚本通过运行期注入的 TLS 证书探测 Chainlink HTTPS readiness。
set -eu

certificate_path=/runtime-state/chainlink/tls/server.crt
test -s "$certificate_path"
curl --fail --silent --show-error \
  --connect-timeout 2 \
  --max-time 4 \
  --resolve chainlink-node:6688:127.0.0.1 \
  --cacert "$certificate_path" \
  https://chainlink-node:6688/readyz >/dev/null
