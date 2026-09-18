# 本文件定义 IPFS Cluster 镜像,固定上游 release 源码并用加固 Go 工具链重建。
ARG GO_BUILDER_IMAGE=invalid.invalid/base/go-builder:required
ARG HTTP_PROXY
ARG HTTPS_PROXY
ARG NO_PROXY

FROM ${GO_BUILDER_IMAGE} AS cluster-build

ARG HTTP_PROXY
ARG HTTPS_PROXY
ARG NO_PROXY

ARG GO_MODULE_PROXY=

# 代理环境下限制 Go 依赖图并发，并固定 HTTP/1.1 传输，避免大规模 libp2p/IPFS 模块下载被截断。
ENV GOMAXPROCS=2
ENV GODEBUG=http2client=0

ADD --chown=1000:1000 --checksum=sha256:5984cc8afe316016b372d50db865b30189f8f8e8d661123ef194dbd1f61c455f https://codeload.github.com/ipfs-cluster/ipfs-cluster/tar.gz/2182e887ef378f906455784396f1011df968822c /tmp/ipfs-cluster.tar.gz
RUN mkdir -p /src/ipfs-cluster \
    && tar -xzf /tmp/ipfs-cluster.tar.gz --strip-components=1 -C /src/ipfs-cluster \
    && rm -f /tmp/ipfs-cluster.tar.gz
WORKDIR /src/ipfs-cluster
COPY entrypoint.go /src/ipfs-cluster-entrypoint.go
RUN --mount=type=cache,target=/go/pkg,uid=1000,gid=1000 \
    --mount=type=cache,target=/tmp/go-cache,uid=1000,gid=1000 \
    export http_proxy="${HTTP_PROXY}" https_proxy="${HTTPS_PROXY}" no_proxy="${NO_PROXY}" \
    && if [ -n "${GO_MODULE_PROXY}" ]; then export GOPROXY="${GO_MODULE_PROXY}"; fi \
    && for attempt in 1 2 3 4 5 6 7 8; do \
        if go get \
            golang.org/x/crypto@v0.55.0 \
            golang.org/x/net@v0.58.0 \
            golang.org/x/text@v0.41.0 \
            google.golang.org/grpc@v1.83.2; then break; fi; \
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
COPY --chown=1000:1000 --from=cluster-build /tmp/chaimir-ipfs-cluster/runtime-state /runtime-state
COPY --chown=1000:1000 --from=cluster-build /tmp/chaimir-ipfs-cluster/workspace /workspace

USER 1000:1000
WORKDIR /workspace
EXPOSE 9094 9095 9096 9097
ENTRYPOINT ["/usr/local/bin/chaimir-ipfs-cluster-entrypoint"]
