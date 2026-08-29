#!/bin/sh
set -eu

db_dir="${CLAMAV_DATABASE_DIR:-/var/lib/clamav}"
mkdir -p "$db_dir" /run/clamav /tmp/clamav

if ! find "$db_dir" -maxdepth 1 \( -name '*.cvd' -o -name '*.cld' \) | grep -q .; then
  freshclam --config-file=/etc/clamav/freshclam.conf --datadir="$db_dir"
fi

# Keep clamd attached to PID 1 so Kubernetes can observe its lifetime and probes.
exec /usr/sbin/clamd --foreground --config-file=/etc/clamav/clamd.conf
