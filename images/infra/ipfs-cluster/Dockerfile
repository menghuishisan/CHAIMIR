# 本文件定义 IPFS Cluster 镜像,固定上游 release 源码并用加固 Go 工具链重建。
ARG GO_BUILDER_IMAGE=invalid.invalid/base/go-builder:required
ARG HTTP_PROXY
ARG HTTPS_PROXY
ARG NO_PROXY

FROM ${GO_BUILDER_IMAGE} AS cluster-build

ARG HTTP_PROXY
ARG HTTPS_PROXY
ARG NO_PROXY

ARG IPFS_CLUSTER_VERSION=v1.1.6
ARG IPFS_CLUSTER_COMMIT=2182e887ef378f906455784396f1011df968822c
ARG GO_MODULE_PROXY=https://goproxy.cn,direct

RUN export http_proxy="${HTTP_PROXY}" https_proxy="${HTTPS_PROXY}" no_proxy="${NO_PROXY}" \
    && for attempt in 1 2 3 4 5 6 7 8; do \
        rm -rf /src/ipfs-cluster; \
        if git -c http.version=HTTP/1.1 clone --branch "${IPFS_CLUSTER_VERSION}" --depth 1 https://github.com/ipfs-cluster/ipfs-cluster.git /src/ipfs-cluster; then break; fi; \
        if [ "${attempt}" -eq 8 ]; then exit 1; fi; \
        sleep $((attempt * 5)); \
    done
WORKDIR /src/ipfs-cluster
COPY entrypoint.go /src/ipfs-cluster-entrypoint.go
RUN export http_proxy="${HTTP_PROXY}" https_proxy="${HTTPS_PROXY}" no_proxy="${NO_PROXY}" \
    && export GOPROXY="${GO_MODULE_PROXY}" \
    && test "$(git rev-parse HEAD)" = "${IPFS_CLUSTER_COMMIT}" \
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
    && CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath -ldflags="-s -w" -o /out/ipfs-cluster-service ./cmd/ipfs-cluster-service \
    && CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath -ldflags="-s -w" -o /out/chaimir-ipfs-cluster-entrypoint /src/ipfs-cluster-entrypoint.go \
    && mkdir -p /tmp/chaimir-ipfs-cluster/runtime-state/ipfs-cluster /tmp/chaimir-ipfs-cluster/workspace

FROM gcr.io/distroless/static-debian12:nonroot@sha256:b7bb25d9f7c31d2bdd1982feb4dafcaf137703c7075dbe2febb41c24212b946f

LABEL org.opencontainers.image.title="infra/ipfs-cluster"
LABEL org.opencontainers.image.description="Chaimir IPFS Cluster coordination image"

# 集群配置和密钥由运行期注入,镜像只准备非 root 运行目录。
COPY --from=cluster-build /out/ipfs-cluster-service /usr/local/bin/ipfs-cluster-service
COPY --from=cluster-build /out/chaimir-ipfs-cluster-entrypoint /usr/local/bin/chaimir-ipfs-cluster-entrypoint
COPY --chown=65532:65532 --from=cluster-build /tmp/chaimir-ipfs-cluster/runtime-state /runtime-state
COPY --chown=65532:65532 --from=cluster-build /tmp/chaimir-ipfs-cluster/workspace /workspace

ENV IPFS_CLUSTER_PATH=/runtime-state/ipfs-cluster
USER nonroot:nonroot
WORKDIR /workspace
EXPOSE 9094 9095 9096
ENTRYPOINT ["/usr/local/bin/chaimir-ipfs-cluster-entrypoint"]
