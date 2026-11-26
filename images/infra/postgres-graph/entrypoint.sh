#!/usr/bin/env sh
# 从平台受控密钥写入数据库 TLS 材料，再交给官方入口初始化或恢复数据。
set -eu

if [ "$#" -eq 0 ]; then set -- postgres; fi
case "$1" in -*) set -- postgres "$@" ;; esac
if [ "$1" != postgres ]; then exec /usr/local/bin/docker-entrypoint.sh "$@"; fi
for argument in "$@"; do
  case "$argument" in
    -\?|--help|--describe-config|-V|--version) exec /usr/local/bin/docker-entrypoint.sh "$@" ;;
  esac
done

: "${POSTGRES_TLS_CERTIFICATE:?PostgreSQL TLS certificate is required}"
: "${POSTGRES_TLS_PRIVATE_KEY:?PostgreSQL TLS private key is required}"
tls_dir=/runtime-state/postgres/tls
umask 077
mkdir -p "$tls_dir"
printf '%s' "$POSTGRES_TLS_CERTIFICATE" > "$tls_dir/server.crt"
printf '%s' "$POSTGRES_TLS_PRIVATE_KEY" > "$tls_dir/server.key"
chmod 600 "$tls_dir/server.crt" "$tls_dir/server.key"
openssl verify -partial_chain -trusted "$tls_dir/server.crt" -purpose sslserver "$tls_dir/server.crt" >/dev/null
certificate_public_key="$(openssl x509 -in "$tls_dir/server.crt" -noout -pubkey)"
private_public_key="$(openssl pkey -in "$tls_dir/server.key" -passin pass: -pubout)"
if [ "$certificate_public_key" != "$private_public_key" ]; then
  echo "PostgreSQL TLS certificate and key do not match" >&2
  exit 2
fi
unset POSTGRES_TLS_CERTIFICATE POSTGRES_TLS_PRIVATE_KEY
exec /usr/local/bin/docker-entrypoint.sh "$@" \
  -c ssl=on \
  -c "ssl_cert_file=$tls_dir/server.crt" \
  -c "ssl_key_file=$tls_dir/server.key"
