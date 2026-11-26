#!/bin/sh
# 本脚本使用运行期 Secret 启动 Redis,保持 Kubernetes command 为 exec 形式。
set -eu
: "${REDIS_PASSWORD:?REDIS_PASSWORD is required}"
exec redis-server --requirepass "${REDIS_PASSWORD}" --appendonly yes
