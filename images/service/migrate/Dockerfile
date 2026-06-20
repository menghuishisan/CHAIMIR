# 本文件构建 Chaimir 部署期 migrate 命令镜像。
FROM golang:1.25-bookworm AS builder

WORKDIR /src

# 先下载依赖,再复制源码,保证依赖层可缓存。
COPY backend/go.mod backend/go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod go mod download
COPY backend/ ./

# migrate 命令用于部署期 migration、授权和 seed 编排。
RUN --mount=type=cache,target=/go/pkg/mod --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=0 GOOS=linux GOMAXPROCS=2 go build -p=2 -trimpath -ldflags="-s -w" -o /out/migrate ./cmd/migrate

FROM gcr.io/distroless/static-debian12:nonroot

LABEL org.opencontainers.image.title="service/migrate"
LABEL org.opencontainers.image.description="Chaimir database migration and seed command image"

WORKDIR /app
COPY --from=builder /out/migrate /app/migrate

USER nonroot:nonroot
ENTRYPOINT ["/app/migrate"]
