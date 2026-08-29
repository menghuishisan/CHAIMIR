# 本文件构建 Chaimir 部署期 migrate 命令镜像。
ARG GO_BUILDER_IMAGE=invalid.invalid/base/go-builder:required
ARG RUNTIME_IMAGE=invalid.invalid/base/runtime-min:required
ARG GO_MODULE_PROXY=

FROM ${GO_BUILDER_IMAGE} AS builder

WORKDIR /src

# 先下载依赖,再复制源码,保证依赖层可缓存。
COPY --chown=1000:1000 backend/go.mod backend/go.sum ./
RUN --mount=type=cache,target=/go/pkg,uid=1000,gid=1000 \
    if [ -n "${GO_MODULE_PROXY}" ]; then go env -w GOPROXY="${GO_MODULE_PROXY}"; fi \
    && go mod download
COPY --chown=1000:1000 backend/ ./
COPY --chown=1000:1000 images/tool /src/acceptance-images/tool
COPY --chown=1000:1000 images/infra /src/acceptance-images/infra
COPY --chown=1000:1000 images/runtime /src/acceptance-images/runtime
COPY --chown=1000:1000 images/judger /src/acceptance-images/judger

# migrate 命令用于部署期 migration、授权和 seed 编排。
RUN --mount=type=cache,target=/go/pkg,uid=1000,gid=1000 --mount=type=cache,target=/tmp/go-cache,uid=1000,gid=1000 \
    CGO_ENABLED=0 GOOS=linux GOMAXPROCS=2 go build -p=2 -trimpath -ldflags="-s -w" -o /out/migrate ./cmd/migrate
RUN find /src/acceptance-images -type f ! -name manifest.yaml -delete && \
    find /src/acceptance-images -type d -empty -delete

FROM ${RUNTIME_IMAGE}

LABEL org.opencontainers.image.title="service/migrate"
LABEL org.opencontainers.image.description="Chaimir database migration and seed command image"

WORKDIR /app
COPY --from=builder /out/migrate /app/migrate
COPY --from=builder /src/acceptance-images /app/images

USER nonroot:nonroot
ENTRYPOINT ["/app/migrate"]
