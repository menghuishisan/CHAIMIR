#!/usr/bin/env sh
# 准备已校验的 Cilium 发布源码，并将安全修复依赖写入模块与 vendor 单一依赖树。
set -eu

source_dir="$1"
source_commit="$2"
cd "$source_dir"

test "$(cat VERSION)" = "1.20.1"
printf '%s %s\n' "$source_commit" "1970-01-01T00:00:00Z" > GIT_VERSION

# Cilium 发布源码携带完整 vendor。仅替换受安全公告影响的三组模块，避免
# `go mod tidy` 为测试包拉取整个依赖图，并保持其余官方 vendor 不变。
go mod edit -require=golang.org/x/text@v0.39.0
go mod edit -require=golang.org/x/crypto@v0.55.0
go mod edit -require=google.golang.org/grpc@v1.83.2
gopath="$(go env GOPATH)"
for module in \
  "golang.org/x/text@v0.39.0" \
  "golang.org/x/crypto@v0.55.0" \
  "google.golang.org/grpc@v1.83.2"; do
  for attempt in 1 2 3 4 5 6 7 8; do
    if go mod download "$module"; then
      break
    fi
    if [ "${attempt}" -eq 8 ]; then
      exit 1
    fi
    sleep $((attempt * 3))
  done
done
rm -rf vendor/golang.org/x/text vendor/golang.org/x/crypto vendor/google.golang.org/grpc
cp -R "$gopath/pkg/mod/golang.org/x/text@v0.39.0" vendor/golang.org/x/text
cp -R "$gopath/pkg/mod/golang.org/x/crypto@v0.55.0" vendor/golang.org/x/crypto
cp -R "$gopath/pkg/mod/google.golang.org/grpc@v1.83.2" vendor/google.golang.org/grpc
sed -i 's/^# golang.org\/x\/text v[0-9].*$/# golang.org\/x\/text v0.39.0/' vendor/modules.txt
sed -i 's/^# golang.org\/x\/crypto v[0-9].*$/# golang.org\/x\/crypto v0.55.0/' vendor/modules.txt
sed -i 's/^# google.golang.org\/grpc v[0-9].*$/# google.golang.org\/grpc v1.83.2/' vendor/modules.txt
grep -q '^# golang.org/x/text v0.39.0$' vendor/modules.txt
grep -q '^# golang.org/x/crypto v0.55.0$' vendor/modules.txt
grep -q '^# google.golang.org/grpc v1.83.2$' vendor/modules.txt
