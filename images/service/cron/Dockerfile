# 本文件构建 Chaimir 受控定时任务镜像,包含 cron 二进制和 PostgreSQL 官方备份客户端。
FROM golang:1.25-bookworm AS builder

WORKDIR /src

# 先下载依赖,再复制源码,保证依赖层可缓存。
COPY backend/go.mod backend/go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod go mod download
COPY backend/ ./

# cron 命令用于部署层真实备份任务,不承载 HTTP 服务。
RUN --mount=type=cache,target=/go/pkg/mod --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=0 GOOS=linux GOMAXPROCS=2 go build -p=2 -trimpath -ldflags="-s -w" -o /out/cron ./cmd/cron

FROM postgres:16-bookworm

LABEL org.opencontainers.image.title="service/cron"
LABEL org.opencontainers.image.description="Chaimir controlled cron task image"

RUN groupadd --system --gid 65532 nonroot \
    && useradd --system --uid 65532 --gid 65532 --home-dir /nonexistent --shell /usr/sbin/nologin nonroot

WORKDIR /app
COPY --from=builder /out/cron /app/cron

USER 65532:65532
ENTRYPOINT ["/app/cron"]
