#!/usr/bin/env bash
# 等待 Ingress 使用的 TLS Secret 就绪，并确认其类型和证书字段完整。
set -euo pipefail

namespace="${1:-chaimir-system}"
secret_name="${2:-chaimir-tls}"
timeout_seconds="${3:-300}"

if [[ ! "$namespace" =~ ^[a-z0-9]([a-z0-9.-]*[a-z0-9])?$ || ! "$secret_name" =~ ^[a-z0-9]([a-z0-9.-]*[a-z0-9])?$ || ! "$timeout_seconds" =~ ^[0-9]+$ ]]; then
  echo "TLS Secret 等待参数非法" >&2
  exit 2
fi

deadline=$((SECONDS + timeout_seconds))
while (( SECONDS <= deadline )); do
  if kubectl -n "$namespace" get secret "$secret_name" >/dev/null 2>&1; then
    secret_type="$(kubectl -n "$namespace" get secret "$secret_name" -o jsonpath='{.type}')"
    cert_data="$(kubectl -n "$namespace" get secret "$secret_name" -o jsonpath='{.data.tls\.crt}')"
    key_data="$(kubectl -n "$namespace" get secret "$secret_name" -o jsonpath='{.data.tls\.key}')"
    if [[ "$secret_type" == "kubernetes.io/tls" && -n "$cert_data" && -n "$key_data" ]]; then
      echo "TLS Secret $namespace/$secret_name 已就绪"
      exit 0
    fi
    echo "TLS Secret $namespace/$secret_name 类型或字段不完整" >&2
    exit 1
  fi
  sleep 5
done

echo "等待 TLS Secret $namespace/$secret_name 超时" >&2
exit 1
