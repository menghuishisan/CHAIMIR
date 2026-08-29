# 本文件定义 IPFS/Kubo 节点镜像,固定官方 Kubo release 源码并最小化运行时层。
ARG GO_BUILDER_IMAGE=invalid.invalid/base/go-builder:required
ARG HTTP_PROXY
ARG HTTPS_PROXY
ARG NO_PROXY

FROM ${GO_BUILDER_IMAGE} AS kubo-build

ARG HTTP_PROXY
ARG HTTPS_PROXY
ARG NO_PROXY

ARG KUBO_VERSION=v0.42.0
ARG KUBO_COMMIT=969853d9609b3a261ef4e41a5eec437188c09ffe
ARG GO_MODULE_PROXY=https://goproxy.cn,direct

RUN export http_proxy="${HTTP_PROXY}" https_proxy="${HTTPS_PROXY}" no_proxy="${NO_PROXY}" \
    && for attempt in 1 2 3 4 5 6 7 8; do \
        rm -rf /src/kubo; \
        if git -c http.version=HTTP/1.1 clone --branch "${KUBO_VERSION}" --depth 1 https://github.com/ipfs/kubo.git /src/kubo; then break; fi; \
        if [ "${attempt}" -eq 8 ]; then exit 1; fi; \
        sleep $((attempt * 5)); \
    done
WORKDIR /src/kubo
RUN export http_proxy="${HTTP_PROXY}" https_proxy="${HTTPS_PROXY}" no_proxy="${NO_PROXY}" \
    && export GOPROXY="${GO_MODULE_PROXY}" \
    && test "$(git rev-parse HEAD)" = "${KUBO_COMMIT}" \
    && for attempt in 1 2 3 4 5 6 7 8; do \
        if go get \
            golang.org/x/crypto@v0.53.0 \
            golang.org/x/net@v0.56.0 \
            golang.org/x/text@v0.39.0 \
            google.golang.org/grpc@v1.82.1; then break; fi; \
        if [ "${attempt}" -eq 8 ]; then exit 1; fi; \
        sleep $((attempt * 5)); \
    done \
    && go mod tidy \
    && CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath -ldflags="-s -w" -o /out/ipfs ./cmd/ipfs

FROM ${GO_BUILDER_IMAGE} AS entrypoint-build

ARG HTTP_PROXY
ARG HTTPS_PROXY
ARG NO_PROXY
ARG GO_MODULE_PROXY=https://goproxy.cn,direct

WORKDIR /src
COPY entrypoint.go .
RUN export http_proxy="${HTTP_PROXY}" https_proxy="${HTTPS_PROXY}" no_proxy="${NO_PROXY}" \
    && CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath -ldflags="-s -w" -o ./chaimir-ipfs-entrypoint ./entrypoint.go

FROM alpine:3.23@sha256:fd791d74b68913cbb027c6546007b3f0d3bc45125f797758156952bc2d6daf40 AS dirs
RUN mkdir -p /out/runtime-state/ipfs /out/workspace

FROM gcr.io/distroless/static-debian12:nonroot@sha256:b7bb25d9f7c31d2bdd1982feb4dafcaf137703c7075dbe2febb41c24212b946f

LABEL org.opencontainers.image.title="infra/ipfs"
LABEL org.opencontainers.image.description="Chaimir IPFS Kubo node image"

# IPFS 数据目录由 runtime-state 卷承载,镜像内不写入业务数据。
COPY --from=kubo-build /out/ipfs /usr/local/bin/ipfs
COPY --from=entrypoint-build /src/chaimir-ipfs-entrypoint /usr/local/bin/chaimir-ipfs-entrypoint
COPY --chown=65532:65532 --from=dirs /out/runtime-state /runtime-state
COPY --chown=65532:65532 --from=dirs /out/workspace /workspace

ENV IPFS_PATH=/runtime-state/ipfs
ENV IPFS_TELEMETRY=off
USER nonroot:nonroot
WORKDIR /workspace
EXPOSE 5001 8080
ENTRYPOINT ["/usr/local/bin/chaimir-ipfs-entrypoint"]
CMD ["daemon", "--migrate=true"]
