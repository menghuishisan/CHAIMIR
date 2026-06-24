# 本文件定义 IPFS/Kubo 节点镜像,固定官方 Kubo release 并最小化运行时层。
ARG GO_BUILDER_IMAGE

FROM ${GO_BUILDER_IMAGE} AS entrypoint-build

WORKDIR /src
COPY entrypoint.go .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath -ldflags="-s -w" -o ./chaimir-ipfs-entrypoint ./entrypoint.go

FROM ipfs/kubo@sha256:6a99314a4fae81abc50a73ccccc0dc4cd49da672a08aca84d25c38c5aa3a8bb3 AS kubo-dist

USER root
RUN ipfs version | grep "0.42.0" \
    && mkdir -p /out/runtime-state/ipfs /out/workspace

FROM gcr.io/distroless/base-debian12:nonroot

LABEL org.opencontainers.image.title="infra/ipfs"
LABEL org.opencontainers.image.description="Chaimir IPFS Kubo node image"

# IPFS 数据目录由 runtime-state 卷承载,镜像内不写入业务数据。
COPY --from=kubo-dist /usr/local/bin/ipfs /usr/local/bin/ipfs
COPY --from=entrypoint-build /src/chaimir-ipfs-entrypoint /usr/local/bin/chaimir-ipfs-entrypoint
COPY --chown=1000:1000 --from=kubo-dist /out/runtime-state /runtime-state
COPY --chown=1000:1000 --from=kubo-dist /out/workspace /workspace

ENV IPFS_PATH=/runtime-state/ipfs
ENV IPFS_TELEMETRY=off
USER 1000:1000
WORKDIR /workspace
EXPOSE 5001 8080
ENTRYPOINT ["/usr/local/bin/chaimir-ipfs-entrypoint"]
CMD ["daemon", "--migrate=true"]
