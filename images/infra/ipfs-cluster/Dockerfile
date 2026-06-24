# 本文件定义 IPFS Cluster 镜像,固定上游 release 源码并用加固 Go 工具链重建。
ARG GO_BUILDER_IMAGE

FROM ${GO_BUILDER_IMAGE} AS cluster-build

ARG IPFS_CLUSTER_VERSION=v1.1.6

RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go install -trimpath -ldflags="-s -w" github.com/ipfs-cluster/ipfs-cluster/cmd/ipfs-cluster-service@${IPFS_CLUSTER_VERSION} \
    && mkdir -p /tmp/chaimir-ipfs-cluster/runtime-state/ipfs-cluster /tmp/chaimir-ipfs-cluster/workspace

FROM gcr.io/distroless/base-debian12:nonroot

LABEL org.opencontainers.image.title="infra/ipfs-cluster"
LABEL org.opencontainers.image.description="Chaimir IPFS Cluster coordination image"

# 集群配置和密钥由运行期注入,镜像只准备非 root 运行目录。
COPY --from=cluster-build /go/bin/ipfs-cluster-service /usr/local/bin/ipfs-cluster-service
COPY --chown=1000:1000 --from=cluster-build /tmp/chaimir-ipfs-cluster/runtime-state /runtime-state
COPY --chown=1000:1000 --from=cluster-build /tmp/chaimir-ipfs-cluster/workspace /workspace

ENV IPFS_CLUSTER_PATH=/runtime-state/ipfs-cluster
USER 1000:1000
WORKDIR /workspace
EXPOSE 9094 9096
ENTRYPOINT ["/usr/local/bin/ipfs-cluster-service"]
CMD ["daemon"]
