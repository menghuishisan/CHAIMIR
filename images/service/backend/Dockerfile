# 本文件构建 Chaimir 后端服务镜像,包含 API server 与独立 RBAC 预配器二进制。
ARG GO_BUILDER_IMAGE=invalid.invalid/base/go-builder:required
ARG RUNTIME_IMAGE=invalid.invalid/base/runtime-min:required
ARG GO_MODULE_PROXY=

FROM ${GO_BUILDER_IMAGE} AS builder

WORKDIR /src

# 先复制依赖清单以复用构建缓存,再复制后端源码。
COPY --chown=1000:1000 backend/go.mod backend/go.sum ./
RUN --mount=type=cache,target=/go/pkg,uid=1000,gid=1000 \
    if [ -n "${GO_MODULE_PROXY}" ]; then go env -w GOPROXY="${GO_MODULE_PROXY}"; fi \
    && go mod download
COPY --chown=1000:1000 backend/ ./

# 使用静态二进制降低运行时依赖,生产部署按 digest 拉取并由 Cosign 校验签名。
RUN --mount=type=cache,target=/go/pkg,uid=1000,gid=1000 --mount=type=cache,target=/tmp/go-cache,uid=1000,gid=1000 \
    CGO_ENABLED=0 GOOS=linux GOMAXPROCS=2 go build -p=2 -trimpath -ldflags="-s -w" -o /out/server ./cmd/server && \
    CGO_ENABLED=0 GOOS=linux GOMAXPROCS=2 go build -p=2 -trimpath -ldflags="-s -w" -o /out/provisioner ./cmd/provisioner

FROM ${RUNTIME_IMAGE}

LABEL org.opencontainers.image.title="service/backend"
LABEL org.opencontainers.image.description="Chaimir backend API and control-plane service"

WORKDIR /app
COPY --from=builder /out/server /app/server
COPY --from=builder /out/provisioner /app/provisioner

USER nonroot:nonroot
EXPOSE 8080
ENTRYPOINT ["/app/server"]
