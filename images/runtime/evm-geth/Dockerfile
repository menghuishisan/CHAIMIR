# 本文件定义 geth 私链运行时镜像,从 go-ethereum 官方源码构建安全二进制。
ARG GO_BUILDER_IMAGE=invalid.invalid/base/go-builder:required
FROM ${GO_BUILDER_IMAGE} AS build

ARG GETH_VERSION=v1.17.4
ARG GETH_SOURCE_SHA256=80b4dee3b89ea40bedc808132befd605208089fdb8fde7981fc3c530596d3a9e
ARG GO_MODULE_PROXY=
ENV CGO_ENABLED=0

# 固定官方 tag 源码包并校验摘要,避免构建依赖 git 服务的瞬时状态。
RUN set -eux; \
    archive=/tmp/geth-source.tar.gz; \
    curl -fL --retry 10 --retry-all-errors --connect-timeout 20 --max-time 180 \
      "https://github.com/ethereum/go-ethereum/archive/refs/tags/${GETH_VERSION}.tar.gz" -o "${archive}"; \
    echo "${GETH_SOURCE_SHA256}  ${archive}" | sha256sum -c -; \
    mkdir -p /src/geth; \
    tar -xzf "${archive}" -C /src/geth --strip-components=1; \
    rm -f "${archive}"
WORKDIR /src/geth
RUN if [ -n "${GO_MODULE_PROXY}" ]; then go env -w GOPROXY="${GO_MODULE_PROXY}"; fi \
    && go get \
        golang.org/x/crypto@v0.53.0 \
        golang.org/x/net@v0.56.0 \
        google.golang.org/grpc@v1.82.1 \
        golang.org/x/text@v0.39.0 \
        go.opentelemetry.io/otel/sdk@v1.43.0 \
    && go mod tidy \
    && go build -trimpath -ldflags "-s -w" -o /out/geth ./cmd/geth

FROM alpine:3.23@sha256:fd791d74b68913cbb027c6546007b3f0d3bc45125f797758156952bc2d6daf40

LABEL org.opencontainers.image.title="runtime/evm-geth"
LABEL org.opencontainers.image.description="Chaimir go-ethereum private-chain runtime built from official source"

USER root
RUN apk add --no-cache bash ca-certificates curl jq \
    && mkdir -p /workspace /runtime-state/geth \
    && chown -R 1000:1000 /workspace /runtime-state

COPY --from=build --chown=1000:1000 /out/geth /usr/local/bin/geth
COPY entrypoint.sh /usr/local/bin/chaimir-geth-entrypoint
COPY chaimir-geth-adapter /usr/local/bin/chaimir-geth-adapter
RUN chmod +x /usr/local/bin/chaimir-geth-entrypoint /usr/local/bin/chaimir-geth-adapter

USER 1000:1000
WORKDIR /workspace
EXPOSE 8545
ENTRYPOINT ["/usr/local/bin/chaimir-geth-entrypoint"]
