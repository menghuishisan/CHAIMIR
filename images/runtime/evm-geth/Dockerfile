# 本文件定义 geth 私链运行时镜像,从 go-ethereum 官方源码构建安全二进制。
ARG GO_BUILDER_IMAGE
ARG CHAIN_TOOLS_IMAGE
FROM ${GO_BUILDER_IMAGE} AS build

ARG GETH_VERSION=v1.17.4
ENV CGO_ENABLED=0

# 固定官方 tag,仅升级已公开修复的 Go 依赖版本后构建 geth。
RUN git clone --branch "${GETH_VERSION}" --depth 1 https://github.com/ethereum/go-ethereum.git /src/geth
WORKDIR /src/geth
RUN go get \
        golang.org/x/crypto@v0.52.0 \
        golang.org/x/net@v0.55.0 \
        google.golang.org/grpc@v1.79.3 \
        go.opentelemetry.io/otel/sdk@v1.43.0 \
    && go mod tidy \
    && go build -trimpath -ldflags "-s -w" -o /out/geth ./cmd/geth

FROM ${CHAIN_TOOLS_IMAGE}

LABEL org.opencontainers.image.title="runtime/evm-geth"
LABEL org.opencontainers.image.description="Chaimir go-ethereum private-chain runtime built from official source"

USER root
RUN apk add --no-cache ca-certificates \
    && mkdir -p /workspace /runtime-state/geth \
    && chown -R 1000:1000 /workspace /runtime-state

COPY --from=build --chown=1000:1000 /out/geth /usr/local/bin/geth
COPY entrypoint.sh /usr/local/bin/chaimir-geth-entrypoint
RUN chmod +x /usr/local/bin/chaimir-geth-entrypoint

USER 1000:1000
WORKDIR /workspace
EXPOSE 8545
ENTRYPOINT ["/usr/local/bin/chaimir-geth-entrypoint"]
