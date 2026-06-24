# 本文件定义 OP Stack op-node 源码重建镜像,完整 L2 工作负载由 M2 WorkloadSpec 组合编排。
ARG GO_BUILDER_IMAGE
ARG RUNTIME_IMAGE=gcr.io/distroless/static-debian12:nonroot

FROM ${GO_BUILDER_IMAGE} AS op-node-build

ARG OP_NODE_VERSION=v1.19.0
ARG OP_NODE_SOURCE_SHA256=195b8a3de77284f620e554cf9462061fd0080adc143cc2784402351390cfab8c
ARG OP_NODE_SOURCE_URL=https://github.com/ethereum-optimism/optimism/archive/refs/tags/op-node/${OP_NODE_VERSION}.tar.gz

USER root
ADD ${OP_NODE_SOURCE_URL} /tmp/op-node-source.tar.gz
RUN echo "${OP_NODE_SOURCE_SHA256}  /tmp/op-node-source.tar.gz" | sha256sum -c - \
    && mkdir -p /src/op-stack \
    && tar -xzf /tmp/op-node-source.tar.gz -C /src/op-stack --strip-components=1 \
    && rm -f /tmp/op-node-source.tar.gz \
    && chown -R 1000:1000 /src/op-stack

USER 1000:1000
WORKDIR /src/op-stack
RUN go get github.com/libp2p/go-libp2p@v0.39.1 \
        github.com/quic-go/quic-go@v0.49.1 \
        go.opentelemetry.io/otel@v1.41.0 \
        go.opentelemetry.io/otel/trace@v1.41.0 \
        go.opentelemetry.io/otel/metric@v1.41.0 \
        go.opentelemetry.io/otel/sdk/metric@v1.41.0 \
        github.com/pion/interceptor@v0.1.39 \
    && sed -i 's|"github.com/libp2p/go-libp2p/p2p/muxer/yamux"|"github.com/libp2p/go-libp2p/p2p/muxer/yamux"\n\t"github.com/libp2p/go-libp2p/p2p/net/swarm"|' op-node/p2p/host.go \
    && sed -i 's|libp2p.MultiaddrResolver(madns.DefaultResolver)|libp2p.MultiaddrResolver(swarm.ResolverFromMaDNS{Resolver: madns.DefaultResolver})|' op-node/p2p/host.go \
    && go mod tidy \
    && CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -p=2 -trimpath -ldflags="-s -w -X github.com/ethereum-optimism/optimism/op-node/version.Version=${OP_NODE_VERSION}-chaimir-secure -X github.com/ethereum-optimism/optimism/op-node/version.Meta=source-rebuild" -o /src/op-node ./op-node/cmd

FROM ${RUNTIME_IMAGE}

LABEL org.opencontainers.image.title="infra/op-stack"
LABEL org.opencontainers.image.description="Chaimir OP Stack op-node source rebuild image"

COPY --from=op-node-build /src/op-node /usr/local/bin/op-node
USER 1000:1000
WORKDIR /workspace
EXPOSE 8545 9003
ENTRYPOINT ["op-node"]
