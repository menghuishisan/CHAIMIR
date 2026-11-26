#!/bin/sh
set -eu

: "${COUCHDB_USER:?COUCHDB_USER is required}"
: "${COUCHDB_PASSWORD:?COUCHDB_PASSWORD is required}"
case "$COUCHDB_USER" in
  ''|*[!A-Za-z0-9_.-]*) echo "COUCHDB_USER contains unsupported characters" >&2; exit 2 ;;
esac
case "$COUCHDB_PASSWORD" in
  ''|*[!A-Za-z0-9_.~-]*) echo "COUCHDB_PASSWORD contains unsupported characters" >&2; exit 2 ;;
esac
if [ "${#COUCHDB_PASSWORD}" -lt 16 ] || [ "${#COUCHDB_PASSWORD}" -gt 128 ]; then
  echo "COUCHDB_PASSWORD length must be between 16 and 128 characters" >&2
  exit 2
fi
umask 077
cat > /opt/couchdb/etc/local.d/10-admin.ini <<EOF
[couchdb]
single_node = true

[admins]
$COUCHDB_USER = $COUCHDB_PASSWORD
EOF
chmod 0600 /opt/couchdb/etc/local.d/10-admin.ini

exec /usr/bin/couchdb
