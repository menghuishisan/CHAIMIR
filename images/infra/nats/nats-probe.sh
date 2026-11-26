#!/bin/sh
# 本脚本检查 NATS 监控端点,供直接 exec 探针调用。
set -eu
exec wget -qO- --timeout=3 http://127.0.0.1:8222/healthz >/dev/null
