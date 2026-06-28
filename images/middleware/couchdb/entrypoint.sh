#!/bin/sh
set -eu

if [ -n "${COUCHDB_USER:-}" ] || [ -n "${COUCHDB_PASSWORD:-}" ]; then
  if [ -z "${COUCHDB_USER:-}" ] || [ -z "${COUCHDB_PASSWORD:-}" ]; then
    echo "COUCHDB_USER and COUCHDB_PASSWORD must be set together" >&2
    exit 1
  fi
  cat > /opt/couchdb/etc/local.d/10-admin.ini <<EOF
[admins]
${COUCHDB_USER} = ${COUCHDB_PASSWORD}
EOF
fi

exec /usr/bin/couchdb
