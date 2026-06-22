# 本文件定义 IPFS/Kubo 节点镜像,固定官方 Kubo release 并最小化运行时层。
ARG GO_BUILDER_IMAGE=chaimir/base/go-builder:dev

FROM ${GO_BUILDER_IMAGE} AS entrypoint-build

WORKDIR /src
COPY entrypoint.go .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath -ldflags="-s -w" -o ./chaimir-ipfs-entrypoint ./entrypoint.go

FROM gcr.io/distroless/base-debian12:debug-nonroot AS kubo-dist

ARG KUBO_VERSION=v0.42.0
ARG KUBO_SHA512=054c38a0cf66f7d738e25085ad62cb3a42d03d4bac329b7dd25c1d71cf18e1ce87d55b1d1b705b04c65210dca9109973579e0eb1cd72f6341ecb3311d840d156

USER root
ADD https://dist.ipfs.tech/kubo/${KUBO_VERSION}/kubo_${KUBO_VERSION}_linux-amd64.tar.gz /tmp/kubo.tar.gz
RUN ["/busybox/sh", "-c", "echo \"${KUBO_SHA512}  /tmp/kubo.tar.gz\" | /busybox/sha512sum -c - \
    && tar -xzf /tmp/kubo.tar.gz -C /tmp \
    && mkdir -p /out/runtime-state/ipfs /out/workspace"]

FROM gcr.io/distroless/base-debian12:nonroot

LABEL org.opencontainers.image.title="infra/ipfs"
LABEL org.opencontainers.image.description="Chaimir IPFS Kubo node image"

# IPFS 数据目录由 runtime-state 卷承载,镜像内不写入业务数据。
COPY --from=kubo-dist /tmp/kubo/ipfs /usr/local/bin/ipfs
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
