# 本文件定义 The Graph 索引节点镜像,复用官方 graph-node 二进制并收紧运行层。
FROM graphprotocol/graph-node:0.44.0@sha256:c14c6d8e2b2b1ed89f6a89babf48d807b18a43e372fda7fb495d3a9050b65b8b AS upstream

FROM registry.chaimir.io/base/go-builder@sha256:fb0a1352643623882f008691aaa1459262c0c5e1030878dbdbc5e916a852a869 AS entrypoint-build

COPY entrypoint.go /src/entrypoint.go
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath -ldflags='-s -w' -o /out/chaimir-thegraph-entrypoint /src/entrypoint.go

FROM alpine:3.23@sha256:fd791d74b68913cbb027c6546007b3f0d3bc45125f797758156952bc2d6daf40 AS dirs
RUN mkdir -p /out/workspace

FROM gcr.io/distroless/cc-debian12:nonroot@sha256:777e96cf322c46bc32aca926c263624c4dc8d7cf37e2fa65ba2c7e697318ebbb

ARG HTTP_PROXY
ARG HTTPS_PROXY
ARG NO_PROXY

LABEL org.opencontainers.image.title="infra/thegraph-node"
LABEL org.opencontainers.image.description="Chaimir The Graph indexing node image"

COPY --from=upstream /usr/local/bin/graph-node /usr/local/bin/graph-node
COPY --from=entrypoint-build /out/chaimir-thegraph-entrypoint /usr/local/bin/chaimir-thegraph-entrypoint
COPY --chown=65532:65532 --from=dirs /out/workspace /workspace

USER nonroot:nonroot
WORKDIR /workspace
EXPOSE 8000 8020 8030 8040
CMD ["graph-node"]
ENTRYPOINT ["/usr/local/bin/chaimir-thegraph-entrypoint"]
