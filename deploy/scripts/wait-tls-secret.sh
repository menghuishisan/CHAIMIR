#!/usr/bin/env bash
# 等待 Ingress 使用的 TLS Secret 就绪，并确认类型、证书有效期、SAN 与私钥匹配。
set -euo pipefail

namespace="${1:-chaimir-system}"
secret_name="${2:-chaimir-tls}"
timeout_seconds="${3:-300}"

if [[ ! "$namespace" =~ ^[a-z0-9]([a-z0-9.-]*[a-z0-9])?$ || ! "$secret_name" =~ ^[a-z0-9]([a-z0-9.-]*[a-z0-9])?$ || ! "$timeout_seconds" =~ ^[0-9]+$ ]]; then
  echo "TLS Secret 等待参数非法" >&2
  exit 2
fi

command -v openssl >/dev/null 2>&1 || { echo "缺少 openssl,无法验证 TLS Secret" >&2; exit 2; }
command -v base64 >/dev/null 2>&1 || { echo "缺少 base64,无法读取 TLS Secret" >&2; exit 2; }

tmp_dir="$(mktemp -d)"
trap 'rm -rf "$tmp_dir"' EXIT

validate_certificate() {
  local cert_path="$tmp_dir/tls.crt"
  local key_path="$tmp_dir/tls.key"
  local cert_pubkey key_pubkey san_output host

  printf '%s' "$cert_data" | base64 --decode > "$cert_path" || return 1
  printf '%s' "$key_data" | base64 --decode > "$key_path" || return 1
  openssl x509 -in "$cert_path" -checkend 86400 -noout >/dev/null 2>&1 || return 1
  san_output="$(openssl x509 -in "$cert_path" -noout -ext subjectAltName 2>/dev/null)" || return 1
  for host in www.chaimir.io tools.chaimir.io; do
    printf '%s\n' "$san_output" | grep -Fq "DNS:$host" || return 1
  done
  cert_pubkey="$(openssl x509 -in "$cert_path" -pubkey -noout | openssl pkey -pubin -outform DER 2>/dev/null | sha256sum | awk '{print $1}')" || return 1
  key_pubkey="$(openssl pkey -in "$key_path" -pubout 2>/dev/null | openssl pkey -pubin -outform DER 2>/dev/null | sha256sum | awk '{print $1}')" || return 1
  [ -n "$cert_pubkey" ] && [ "$cert_pubkey" = "$key_pubkey" ]
}

deadline=$((SECONDS + timeout_seconds))
while (( SECONDS <= deadline )); do
  if kubectl --request-timeout=10s -n "$namespace" get secret "$secret_name" >/dev/null 2>&1; then
    secret_type="$(kubectl --request-timeout=10s -n "$namespace" get secret "$secret_name" -o jsonpath='{.type}')"
    cert_data="$(kubectl --request-timeout=10s -n "$namespace" get secret "$secret_name" -o jsonpath='{.data.tls\.crt}')"
    key_data="$(kubectl --request-timeout=10s -n "$namespace" get secret "$secret_name" -o jsonpath='{.data.tls\.key}')"
    if [[ "$secret_type" == "kubernetes.io/tls" && -n "$cert_data" && -n "$key_data" ]]; then
      if validate_certificate; then
        echo "TLS Secret $namespace/$secret_name 已就绪"
        exit 0
      fi
      echo "TLS Secret $namespace/$secret_name 的证书仍未通过有效期、SAN 或私钥匹配校验" >&2
    else
      echo "TLS Secret $namespace/$secret_name 类型或字段不完整" >&2
    fi
  fi
  sleep 5
done

echo "等待 TLS Secret $namespace/$secret_name 超时" >&2
exit 1
