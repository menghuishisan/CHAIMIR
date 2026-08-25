# 本文件定义 IPFS Cluster 镜像,固定上游 release 源码并用加固 Go 工具链重建。
ARG GO_BUILDER_IMAGE=invalid.invalid/base/go-builder:required

FROM ${GO_BUILDER_IMAGE} AS cluster-build

ARG IPFS_CLUSTER_VERSION=v1.1.6
ARG IPFS_CLUSTER_COMMIT=2182e887ef378f906455784396f1011df968822c

RUN git clone --branch "${IPFS_CLUSTER_VERSION}" --depth 1 https://github.com/ipfs-cluster/ipfs-cluster.git /src/ipfs-cluster
WORKDIR /src/ipfs-cluster
COPY entrypoint.go /src/ipfs-cluster-entrypoint.go
RUN test "$(git rev-parse HEAD)" = "${IPFS_CLUSTER_COMMIT}" \
    && go get \
        golang.org/x/crypto@v0.53.0 \
        golang.org/x/net@v0.56.0 \
        golang.org/x/text@v0.39.0 \
        google.golang.org/grpc@v1.82.1 \
    && go mod tidy \
    && CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath -ldflags="-s -w" -o /out/ipfs-cluster-service ./cmd/ipfs-cluster-service \
    && CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath -ldflags="-s -w" -o /out/chaimir-ipfs-cluster-entrypoint /src/ipfs-cluster-entrypoint.go \
    && mkdir -p /tmp/chaimir-ipfs-cluster/runtime-state/ipfs-cluster /tmp/chaimir-ipfs-cluster/workspace

FROM debian:13-slim@sha256:abc9cb88a5587630d7f915f47b23b0668fe250fbfc6457aa4d52b534c1bbf73f

LABEL org.opencontainers.image.title="infra/ipfs-cluster"
LABEL org.opencontainers.image.description="Chaimir IPFS Cluster coordination image"

RUN mkdir -p /etc/ssl/certs /runtime-state/ipfs-cluster /workspace \
    && chown -R 1000:1000 /runtime-state /workspace

COPY --from=cluster-build /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/ca-certificates.crt

# 集群配置和密钥由运行期注入,镜像只准备非 root 运行目录。
COPY --from=cluster-build /out/ipfs-cluster-service /usr/local/bin/ipfs-cluster-service
COPY --from=cluster-build /out/chaimir-ipfs-cluster-entrypoint /usr/local/bin/chaimir-ipfs-cluster-entrypoint
COPY --chown=1000:1000 --from=cluster-build /tmp/chaimir-ipfs-cluster/runtime-state /runtime-state
COPY --chown=1000:1000 --from=cluster-build /tmp/chaimir-ipfs-cluster/workspace /workspace

ENV IPFS_CLUSTER_PATH=/runtime-state/ipfs-cluster
USER 1000:1000
WORKDIR /workspace
EXPOSE 9094 9095 9096
ENTRYPOINT ["/usr/local/bin/chaimir-ipfs-cluster-entrypoint"]
