# 本文件构建 Chaimir 后端服务镜像,最终产物只包含 server 二进制。
ARG GO_BUILDER_IMAGE=golang:1.25-bookworm
ARG RUNTIME_IMAGE=gcr.io/distroless/static-debian12:nonroot

FROM ${GO_BUILDER_IMAGE} AS builder

WORKDIR /src

# 先复制依赖清单以复用构建缓存,再复制后端源码。
COPY --chown=1000:1000 backend/go.mod backend/go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod,uid=1000,gid=1000 go mod download
COPY --chown=1000:1000 backend/ ./

# 使用静态二进制降低运行时依赖,生产部署按 digest 拉取并由 Cosign 校验签名。
RUN --mount=type=cache,target=/go/pkg/mod,uid=1000,gid=1000 --mount=type=cache,target=/tmp/go-cache,uid=1000,gid=1000 \
    CGO_ENABLED=0 GOOS=linux GOMAXPROCS=2 go build -p=2 -trimpath -ldflags="-s -w" -o /out/server ./cmd/server

FROM ${RUNTIME_IMAGE}

LABEL org.opencontainers.image.title="service/backend"
LABEL org.opencontainers.image.description="Chaimir backend API and control-plane service"

WORKDIR /app
COPY --from=builder /out/server /app/server

USER nonroot:nonroot
EXPOSE 8080
ENTRYPOINT ["/app/server"]
