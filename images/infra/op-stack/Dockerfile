# 本文件定义 OP Stack op-node 源码重建镜像,完整 L2 工作负载由 M2 WorkloadSpec 组合编排。
ARG GO_BUILDER_IMAGE=invalid.invalid/base/go-builder:required
ARG RUNTIME_IMAGE=invalid.invalid/base/runtime-min:required

FROM ${GO_BUILDER_IMAGE} AS op-node-build

ARG OP_NODE_VERSION=v1.19.2
ARG OP_NODE_SOURCE_SHA256=0ff5ab74e76b35f0225c6e417ec9fae1025fff46e104a77341593919ae4d17de
ARG OP_NODE_SOURCE_URL=https://github.com/ethereum-optimism/optimism/archive/da197e45ed44b9fca258b3b0d0709e8dfca1c7cd.tar.gz
ARG SUPERCHAIN_REGISTRY_SOURCE_SHA256=08600ee12e2d8933275813e4ef74e30929f95c249f6bce714c9c337527c20d4c
ARG SUPERCHAIN_REGISTRY_SOURCE_URL=https://github.com/ethereum-optimism/superchain-registry/archive/d84b53b7ee008f863c1d3bbb49e06f713f713978.tar.gz
ARG YQ_SOURCE_URL=https://github.com/mikefarah/yq/releases/download/v4.44.3/yq_linux_amd64
ARG YQ_SOURCE_SHA256=a2c097180dd884a8d50c956ee16a9cec070f30a7947cf4ebf87d5f36213e9ed7
ARG GO_MODULE_PROXY=

USER root
ADD ${OP_NODE_SOURCE_URL} /tmp/op-node-source.tar.gz
ADD ${SUPERCHAIN_REGISTRY_SOURCE_URL} /tmp/superchain-registry-source.tar.gz
RUN echo "${OP_NODE_SOURCE_SHA256}  /tmp/op-node-source.tar.gz" | sha256sum -c - \
    && echo "${SUPERCHAIN_REGISTRY_SOURCE_SHA256}  /tmp/superchain-registry-source.tar.gz" | sha256sum -c - \
    && apk add --no-cache bash jq zip ca-certificates curl \
    && curl --fail --silent --show-error --location --retry 5 --retry-all-errors --connect-timeout 20 --max-time 120 "${YQ_SOURCE_URL}" -o /usr/local/bin/yq \
    && echo "${YQ_SOURCE_SHA256}  /usr/local/bin/yq" | sha256sum -c - \
    && chmod 0755 /usr/local/bin/yq \
    && mkdir -p /src/op-stack \
    && tar -xzf /tmp/op-node-source.tar.gz -C /src/op-stack --strip-components=1 \
    && mkdir -p /src/op-stack/superchain-registry \
    && tar -xzf /tmp/superchain-registry-source.tar.gz -C /src/op-stack/superchain-registry --strip-components=1 \
    && rm -f /tmp/op-node-source.tar.gz /tmp/superchain-registry-source.tar.gz \
    && chown -R 1000:1000 /src/op-stack

USER 1000:1000
WORKDIR /src/op-stack
RUN if [ -n "${GO_MODULE_PROXY}" ]; then go env -w GOPROXY="${GO_MODULE_PROXY}"; fi \
    && bash op-core/superchain/sync-superchain.sh \
    && sed -i 's|"github.com/libp2p/go-libp2p/p2p/muxer/yamux"|"github.com/libp2p/go-libp2p/p2p/muxer/yamux"\n\t"github.com/libp2p/go-libp2p/p2p/net/swarm"|' op-node/p2p/host.go \
    && sed -i 's|libp2p.MultiaddrResolver(madns.DefaultResolver)|libp2p.MultiaddrResolver(swarm.ResolverFromMaDNS{Resolver: madns.DefaultResolver})|' op-node/p2p/host.go \
    && for attempt in 1 2 3 4 5; do \
        if go get github.com/libp2p/go-libp2p@v0.39.1 \
            github.com/quic-go/quic-go@v0.49.1 \
            github.com/pion/interceptor@v0.1.39 \
            golang.org/x/mod@v0.40.0 \
            golang.org/x/text@v0.41.0 \
            google.golang.org/grpc@v1.82.1 \
            && go mod tidy \
            && CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -p=2 -trimpath -ldflags="-s -w -X github.com/ethereum-optimism/optimism/op-node/version.Version=${OP_NODE_VERSION}-chaimir-secure -X github.com/ethereum-optimism/optimism/op-node/version.Meta=source-rebuild" -o /src/op-node ./op-node/cmd; then \
            break; \
        fi; \
        if [ "${attempt}" -eq 5 ]; then exit 1; fi; \
        sleep $((attempt * 5)); \
    done

COPY entrypoint.go /src/chaimir-op-node-entrypoint.go
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath -ldflags="-s -w" -o /src/chaimir-op-node-entrypoint /src/chaimir-op-node-entrypoint.go \
    && mkdir -p /out/runtime-state

FROM ${RUNTIME_IMAGE}

LABEL org.opencontainers.image.title="infra/op-stack"
LABEL org.opencontainers.image.description="Chaimir OP Stack op-node source rebuild image"

COPY --from=op-node-build /src/op-node /usr/local/bin/op-node
COPY --from=op-node-build /src/chaimir-op-node-entrypoint /usr/local/bin/chaimir-op-node-entrypoint
COPY --from=op-node-build --chown=1000:1000 /out/runtime-state /runtime-state
USER 1000:1000
WORKDIR /workspace
EXPOSE 8545 9003
ENTRYPOINT ["/usr/local/bin/chaimir-op-node-entrypoint"]
