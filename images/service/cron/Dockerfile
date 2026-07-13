# 本文件构建 Chaimir 受控定时任务镜像,包含 cron 二进制和 PostgreSQL 官方备份客户端。
ARG GO_BUILDER_IMAGE=invalid.invalid/base/go-builder:required
ARG PG_CLIENT_IMAGE=invalid.invalid/middleware/postgres:required

FROM ${GO_BUILDER_IMAGE} AS builder

WORKDIR /src

# 先下载依赖,再复制源码,保证依赖层可缓存。
COPY --chown=1000:1000 backend/go.mod backend/go.sum ./
RUN --mount=type=cache,target=/go/pkg,uid=1000,gid=1000 go mod download
COPY --chown=1000:1000 backend/ ./

# cron 命令用于部署层真实备份任务,不承载 HTTP 服务。
RUN --mount=type=cache,target=/go/pkg,uid=1000,gid=1000 --mount=type=cache,target=/tmp/go-cache,uid=1000,gid=1000 \
    CGO_ENABLED=0 GOOS=linux GOMAXPROCS=2 go build -p=2 -trimpath -ldflags="-s -w" -o /out/cron ./cmd/cron

FROM ${PG_CLIENT_IMAGE}

LABEL org.opencontainers.image.title="service/cron"
LABEL org.opencontainers.image.description="Chaimir controlled cron task image"

USER root
RUN apk upgrade --no-cache \
    && rm -f /usr/local/bin/gosu \
    && addgroup -S -g 65532 nonroot \
    && adduser -S -u 65532 -G nonroot -h /nonexistent -s /sbin/nologin nonroot

WORKDIR /app
COPY --from=builder /out/cron /app/cron

USER 65532:65532
ENV HOME=/nonexistent
ENTRYPOINT ["/app/cron"]
