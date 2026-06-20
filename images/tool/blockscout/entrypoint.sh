#!/usr/bin/env sh
# This entrypoint validates Blockscout runtime dependencies before starting the web service.
set -eu

: "${DATABASE_URL:?DATABASE_URL is required}"
: "${ETHEREUM_JSONRPC_HTTP_URL:?ETHEREUM_JSONRPC_HTTP_URL is required}"

export PORT="${PORT:-4000}"
export SECRET_KEY_BASE="${SECRET_KEY_BASE:-$(openssl rand -hex 64)}"
export ECTO_USE_SSL="${ECTO_USE_SSL:-false}"
export ETHEREUM_JSONRPC_VARIANT="${ETHEREUM_JSONRPC_VARIANT:-geth}"
export ETHEREUM_JSONRPC_TRACE_URL="${ETHEREUM_JSONRPC_TRACE_URL:-$ETHEREUM_JSONRPC_HTTP_URL}"
export INDEXER_DISABLE_PENDING_TRANSACTIONS_FETCHER="${INDEXER_DISABLE_PENDING_TRANSACTIONS_FETCHER:-true}"
export DISABLE_EXCHANGE_RATES="${DISABLE_EXCHANGE_RATES:-true}"
export MIX_ENV="${MIX_ENV:-prod}"
export BLOCKSCOUT_DEPENDENCY_TIMEOUT_SECONDS="${BLOCKSCOUT_DEPENDENCY_TIMEOUT_SECONDS:-180}"
export BLOCKSCOUT_DEPENDENCY_POLL_SECONDS="${BLOCKSCOUT_DEPENDENCY_POLL_SECONDS:-2}"

url_host_port() {
  url="$1"
  default_port="$2"
  target="${url#*://}"
  target="${target##*@}"
  target="${target%%/*}"
  target="${target%%\?*}"
  host="${target%:*}"
  port="${target##*:}"
  if [ "$host" = "$target" ]; then
    port="$default_port"
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

set -- $(url_host_port "$DATABASE_URL" 5432)
wait_for_tcp "database" "$1" "$2"
set -- $(url_host_port "$ETHEREUM_JSONRPC_HTTP_URL" 8545)
wait_for_tcp "jsonrpc" "$1" "$2"

/app/bin/blockscout eval "Explorer.ReleaseTasks.create_and_migrate()"
exec /app/bin/blockscout start
