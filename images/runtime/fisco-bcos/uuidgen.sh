#!/usr/bin/env sh
# 本脚本为官方 build_chain 的 UUID 查询提供最小内核随机源适配。
set -eu

exec cat /proc/sys/kernel/random/uuid
