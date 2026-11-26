#!/bin/sh
# 本脚本使用运行期 Secret 启动 NATS,保留直接 exec 进程语义。
set -eu
: "${NATS_TOKEN:?NATS_TOKEN is required}"
exec /usr/local/bin/nats-server -js -sd /data -m 8222 --auth "${NATS_TOKEN}"
