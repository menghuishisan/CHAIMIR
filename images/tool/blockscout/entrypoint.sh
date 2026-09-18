#!/usr/bin/env sh
# 本入口在启动 Web 服务前校验 Blockscout 的运行时依赖。
set -eu

: "${CHAIMIR_BLOCKSCOUT_DATABASE_ADDRESS:?CHAIMIR_BLOCKSCOUT_DATABASE_ADDRESS is required}"
: "${CHAIMIR_BLOCKSCOUT_DATABASE_NAME:?CHAIMIR_BLOCKSCOUT_DATABASE_NAME is required}"
: "${CHAIMIR_BLOCKSCOUT_DATABASE_USER:?CHAIMIR_BLOCKSCOUT_DATABASE_USER is required}"
: "${ETHEREUM_JSONRPC_HTTP_URL:?ETHEREUM_JSONRPC_HTTP_URL is required}"
: "${ETHEREUM_JSONRPC_TRACE_URL:?ETHEREUM_JSONRPC_TRACE_URL is required}"
: "${SECRET_KEY_BASE:?SECRET_KEY_BASE is required}"
: "${POSTGRES_PASSWORD:?POSTGRES_PASSWORD is required}"
: "${PORT:?PORT is required}"
: "${ECTO_SSL_MODE:?ECTO_SSL_MODE is required}"
: "${ETHEREUM_JSONRPC_VARIANT:?ETHEREUM_JSONRPC_VARIANT is required}"
: "${INDEXER_DISABLE_PENDING_TRANSACTIONS_FETCHER:?INDEXER_DISABLE_PENDING_TRANSACTIONS_FETCHER is required}"
: "${DISABLE_EXCHANGE_RATES:?DISABLE_EXCHANGE_RATES is required}"
: "${DISABLE_WEBAPP:?DISABLE_WEBAPP is required}"
: "${MIX_ENV:?MIX_ENV is required}"
: "${BLOCKSCOUT_DEPENDENCY_TIMEOUT_SECONDS:?BLOCKSCOUT_DEPENDENCY_TIMEOUT_SECONDS is required}"
: "${BLOCKSCOUT_DEPENDENCY_POLL_SECONDS:?BLOCKSCOUT_DEPENDENCY_POLL_SECONDS is required}"

case "$PORT" in
  ''|*[!0-9]*) echo "PORT must be a valid TCP port" >&2; exit 2 ;;
esac
if [ "$PORT" -lt 1 ] || [ "$PORT" -gt 65535 ]; then
  echo "PORT must be in 1-65535" >&2
  exit 2
fi
case "$BLOCKSCOUT_DEPENDENCY_TIMEOUT_SECONDS" in
  ''|*[!0-9]*) echo "BLOCKSCOUT_DEPENDENCY_TIMEOUT_SECONDS must be a positive integer" >&2; exit 2 ;;
esac
if [ "$BLOCKSCOUT_DEPENDENCY_TIMEOUT_SECONDS" -lt 1 ]; then
  echo "BLOCKSCOUT_DEPENDENCY_TIMEOUT_SECONDS must be a positive integer" >&2
  exit 2
fi
case "$BLOCKSCOUT_DEPENDENCY_POLL_SECONDS" in
  ''|*[!0-9]*) echo "BLOCKSCOUT_DEPENDENCY_POLL_SECONDS must be a positive integer" >&2; exit 2 ;;
esac
if [ "$BLOCKSCOUT_DEPENDENCY_POLL_SECONDS" -lt 1 ]; then
  echo "BLOCKSCOUT_DEPENDENCY_POLL_SECONDS must be a positive integer" >&2
  exit 2
fi

database_name="${CHAIMIR_BLOCKSCOUT_DATABASE_NAME}"
database_user="${CHAIMIR_BLOCKSCOUT_DATABASE_USER}"
case "${database_name}" in *[!A-Za-z0-9_.-]*|'') echo "CHAIMIR_BLOCKSCOUT_DATABASE_NAME is invalid" >&2; exit 2;; esac
case "${database_user}" in *[!A-Za-z0-9_.-]*|'') echo "CHAIMIR_BLOCKSCOUT_DATABASE_USER is invalid" >&2; exit 2;; esac
case "${POSTGRES_PASSWORD}" in *[!A-Za-z0-9_.~-]*|'') echo "POSTGRES_PASSWORD is invalid" >&2; exit 2;; esac
export DATABASE_URL="postgresql://${database_user}:${POSTGRES_PASSWORD}@${CHAIMIR_BLOCKSCOUT_DATABASE_ADDRESS}/${database_name}"

export PORT="${PORT}"
export ECTO_SSL_MODE="${ECTO_SSL_MODE}"
export DISABLE_WEBAPP="${DISABLE_WEBAPP}"
export ETHEREUM_JSONRPC_VARIANT="${ETHEREUM_JSONRPC_VARIANT}"
export ETHEREUM_JSONRPC_TRACE_URL="${ETHEREUM_JSONRPC_TRACE_URL}"
export INDEXER_DISABLE_PENDING_TRANSACTIONS_FETCHER="${INDEXER_DISABLE_PENDING_TRANSACTIONS_FETCHER}"
export DISABLE_EXCHANGE_RATES="${DISABLE_EXCHANGE_RATES}"
export MIX_ENV="${MIX_ENV}"
export BLOCKSCOUT_DEPENDENCY_TIMEOUT_SECONDS="${BLOCKSCOUT_DEPENDENCY_TIMEOUT_SECONDS}"
export BLOCKSCOUT_DEPENDENCY_POLL_SECONDS="${BLOCKSCOUT_DEPENDENCY_POLL_SECONDS}"

tcp_host_port() {
  target="$1"
  case "$target" in
    *://*|*@*|*/*|*\?*|*\#*)
      echo "dependency address must be an uncredentialed host:port: $target" >&2
      return 2
      ;;
  esac
  host="${target%:*}"
  port="${target##*:}"
  if [ "$host" = "$target" ] || [ -z "$host" ] || [ -z "$port" ]; then
    echo "dependency address must include an explicit host and port: $target" >&2
    return 2
  fi
  case "$port" in
    ''|*[!0-9]*)
      echo "dependency address port is invalid: $target" >&2
      return 2
      ;;
  esac
  if [ "$port" -lt 1 ] || [ "$port" -gt 65535 ]; then
    echo "dependency address port is outside 1-65535: $target" >&2
    return 2
  fi
  printf '%s %s\n' "$host" "$port"
}

http_url_host_port() {
  url="$1"
  case "$url" in
    http://*|https://*) ;;
    *)
      echo "dependency URL must use HTTP(S): $url" >&2
      return 2
      ;;
  esac
  target="${url#*://}"
  case "$target" in
    *@*)
      echo "dependency URL must not contain credentials: $url" >&2
      return 2
      ;;
  esac
  target="${target%%/*}"
  target="${target%%\?*}"
  target="${target%%\#*}"
  host="${target%:*}"
  port="${target##*:}"
  if [ "$host" = "$target" ] || [ -z "$host" ] || [ -z "$port" ]; then
    echo "dependency URL must include an explicit host and port: $url" >&2
    return 2
  fi
  case "$port" in
    ''|*[!0-9]*)
      echo "dependency URL port is invalid: $url" >&2
      return 2
      ;;
  esac
  if [ "$port" -lt 1 ] || [ "$port" -gt 65535 ]; then
    echo "dependency URL port is outside 1-65535: $url" >&2
    return 2
  fi
  printf '%s %s\n' "$host" "$port"
}

wait_for_tcp() {
  name="$1"
  host="$2"
  port="$3"
  deadline=$(( $(date +%s) + BLOCKSCOUT_DEPENDENCY_TIMEOUT_SECONDS ))
  while [ "$(date +%s)" -le "$deadline" ]; do
    if getent hosts "$host" >/dev/null 2>&1 && nc -z "$host" "$port" >/dev/null 2>&1; then
      echo "dependency_ready name=$name host=$host port=$port"
      return 0
    fi
    sleep "$BLOCKSCOUT_DEPENDENCY_POLL_SECONDS"
  done
  echo "dependency_unavailable name=$name host=$host port=$port timeout_seconds=$BLOCKSCOUT_DEPENDENCY_TIMEOUT_SECONDS" >&2
  return 1
}

database_target="$(tcp_host_port "$CHAIMIR_BLOCKSCOUT_DATABASE_ADDRESS")"
set -- $database_target
wait_for_tcp "database" "$1" "$2"
rpc_target="$(http_url_host_port "$ETHEREUM_JSONRPC_HTTP_URL")"
set -- $rpc_target
wait_for_tcp "jsonrpc" "$1" "$2"
trace_target="$(http_url_host_port "$ETHEREUM_JSONRPC_TRACE_URL")"
set -- $trace_target
wait_for_tcp "trace-jsonrpc" "$1" "$2"

/app/bin/blockscout eval "Explorer.ReleaseTasks.create_and_migrate()"
exec /app/bin/blockscout start
