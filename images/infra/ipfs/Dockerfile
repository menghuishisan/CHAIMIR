# 本文件定义 IPFS/Kubo 节点镜像,固定官方 Kubo release 源码并最小化运行时层。
ARG GO_BUILDER_IMAGE

FROM ${GO_BUILDER_IMAGE} AS kubo-build

ARG KUBO_VERSION=v0.42.0
ARG KUBO_COMMIT=969853d9609b3a261ef4e41a5eec437188c09ffe

RUN git clone --branch "${KUBO_VERSION}" --depth 1 https://github.com/ipfs/kubo.git /src/kubo
WORKDIR /src/kubo
RUN test "$(git rev-parse HEAD)" = "${KUBO_COMMIT}" \
    && go get \
        golang.org/x/crypto@v0.52.0 \
        golang.org/x/net@v0.55.0 \
    && go mod tidy \
    && CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath -ldflags="-s -w" -o /out/ipfs ./cmd/ipfs

FROM ${GO_BUILDER_IMAGE} AS entrypoint-build

WORKDIR /src
COPY entrypoint.go .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath -ldflags="-s -w" -o ./chaimir-ipfs-entrypoint ./entrypoint.go

FROM alpine:3.23 AS dirs
RUN mkdir -p /out/runtime-state/ipfs /out/workspace

FROM gcr.io/distroless/base-debian12:nonroot

LABEL org.opencontainers.image.title="infra/ipfs"
LABEL org.opencontainers.image.description="Chaimir IPFS Kubo node image"

# IPFS 数据目录由 runtime-state 卷承载,镜像内不写入业务数据。
COPY --from=kubo-build /out/ipfs /usr/local/bin/ipfs
COPY --from=entrypoint-build /src/chaimir-ipfs-entrypoint /usr/local/bin/chaimir-ipfs-entrypoint
COPY --chown=1000:1000 --from=dirs /out/runtime-state /runtime-state
COPY --chown=1000:1000 --from=dirs /out/workspace /workspace

ENV IPFS_PATH=/runtime-state/ipfs
ENV IPFS_TELEMETRY=off
USER 1000:1000
WORKDIR /workspace
EXPOSE 5001 8080
ENTRYPOINT ["/usr/local/bin/chaimir-ipfs-entrypoint"]
CMD ["daemon", "--migrate=true"]
