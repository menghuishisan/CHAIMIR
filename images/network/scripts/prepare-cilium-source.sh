#!/usr/bin/env sh
# 准备已校验的 Cilium 发布源码，并将已修复的 x/text 写入模块与 vendor 单一依赖树。
set -eu

source_dir="$1"
source_commit="$2"
cd "$source_dir"

test "$(cat VERSION)" = "1.20.0"
printf '%s %s\n' "$source_commit" "1970-01-01T00:00:00Z" > GIT_VERSION

# Cilium 的发布源码携带 vendor；只改 go.mod 会被 vendored 旧版本覆盖，必须同步重建 vendor。
go mod edit -require=golang.org/x/text@v0.39.0
go mod tidy
go mod vendor
grep -q '^# golang.org/x/text v0.39.0$' vendor/modules.txt
