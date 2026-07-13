# 本文件定义 IPFS Cluster 镜像,固定上游 release 源码并用加固 Go 工具链重建。
ARG GO_BUILDER_IMAGE=invalid.invalid/base/go-builder:required

FROM ${GO_BUILDER_IMAGE} AS cluster-build

ARG IPFS_CLUSTER_VERSION=v1.1.6
ARG IPFS_CLUSTER_COMMIT=2182e887ef378f906455784396f1011df968822c

RUN git clone --branch "${IPFS_CLUSTER_VERSION}" --depth 1 https://github.com/ipfs-cluster/ipfs-cluster.git /src/ipfs-cluster
WORKDIR /src/ipfs-cluster
RUN test "$(git rev-parse HEAD)" = "${IPFS_CLUSTER_COMMIT}" \
    && go get \
        golang.org/x/crypto@v0.52.0 \
        golang.org/x/net@v0.55.0 \
    && go mod tidy \
    && CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath -ldflags="-s -w" -o /out/ipfs-cluster-service ./cmd/ipfs-cluster-service \
    && mkdir -p /tmp/chaimir-ipfs-cluster/runtime-state/ipfs-cluster /tmp/chaimir-ipfs-cluster/workspace

FROM gcr.io/distroless/base-debian12:nonroot@sha256:4b5196599229a5cf312a676cfe1ee8587ecf2371dcc22620f8c7a66d77d125c8

LABEL org.opencontainers.image.title="infra/ipfs-cluster"
LABEL org.opencontainers.image.description="Chaimir IPFS Cluster coordination image"

# 集群配置和密钥由运行期注入,镜像只准备非 root 运行目录。
COPY --from=cluster-build /out/ipfs-cluster-service /usr/local/bin/ipfs-cluster-service
COPY --chown=1000:1000 --from=cluster-build /tmp/chaimir-ipfs-cluster/runtime-state /runtime-state
COPY --chown=1000:1000 --from=cluster-build /tmp/chaimir-ipfs-cluster/workspace /workspace

ENV IPFS_CLUSTER_PATH=/runtime-state/ipfs-cluster
USER 1000:1000
WORKDIR /workspace
EXPOSE 9094 9096
ENTRYPOINT ["/usr/local/bin/ipfs-cluster-service"]
CMD ["daemon"]
