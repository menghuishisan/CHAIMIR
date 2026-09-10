#!/bin/sh
# 本脚本以运行期 Secret 探测 Redis,不通过 shell 字符串拼接探针命令。
set -eu
: "${REDIS_PASSWORD:?REDIS_PASSWORD is required}"
test "$(redis-cli --no-auth-warning -h 127.0.0.1 -p 6379 -a "${REDIS_PASSWORD}" ping)" = PONG
