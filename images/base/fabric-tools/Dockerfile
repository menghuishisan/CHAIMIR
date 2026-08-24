# 本文件从 Hyperledger Fabric v2.5.16 官方源码构建平台共用工具基座。
ARG GO_BUILDER_IMAGE=invalid.invalid/base/go-builder:required
FROM ${GO_BUILDER_IMAGE} AS build

ARG FABRIC_SOURCE_SHA256=dbcd31de33eeff358fe923d5cbb669b33cb653ef10ae2e6505555e372ae26d82
ARG FABRIC_SOURCE_URL=https://github.com/hyperledger/fabric/archive/f871cf92a026aba7b12e6f06d71ded3e6e659d71.tar.gz

USER root
ADD ${FABRIC_SOURCE_URL} /tmp/fabric-source.tar.gz
RUN echo "${FABRIC_SOURCE_SHA256}  /tmp/fabric-source.tar.gz" | sha256sum -c - \
    && mkdir -p /src/fabric /out/bin /out/config \
    && tar -xzf /tmp/fabric-source.tar.gz -C /src/fabric --strip-components=1 \
    && rm -f /tmp/fabric-source.tar.gz \
    && chown -R 1000:1000 /src/fabric /out

USER 1000:1000
WORKDIR /src/fabric
# 代理环境下限制 Go 构建并发,避免完整 Fabric 依赖图同时建立过多外部连接导致模块下载被截断。
ENV GOMAXPROCS=2
ARG GO_MODULE_PROXY=
RUN --mount=type=cache,target=/go/pkg,uid=1000,gid=1000 \
    --mount=type=cache,target=/tmp/go-cache,uid=1000,gid=1000 \
    if [ -n "${GO_MODULE_PROXY}" ]; then go env -w GOPROXY="${GO_MODULE_PROXY}"; fi \
    && go mod download \
    && go get golang.org/x/text@v0.39.0 google.golang.org/grpc@v1.82.1 \
    && go mod tidy \
    && go mod vendor \
    && CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath -ldflags="-s -w" -o /out/bin/peer ./cmd/peer \
    && CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath -ldflags="-s -w" -o /out/bin/orderer ./cmd/orderer \
    && CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath -ldflags="-s -w" -o /out/bin/configtxgen ./cmd/configtxgen \
    && CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath -ldflags="-s -w" -o /out/bin/configtxlator ./cmd/configtxlator \
    && CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath -ldflags="-s -w" -o /out/bin/cryptogen ./cmd/cryptogen \
    && CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath -ldflags="-s -w" -o /out/bin/discover ./cmd/discover \
    && CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath -ldflags="-s -w" -o /out/bin/ledgerutil ./cmd/ledgerutil \
    && CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath -ldflags="-s -w" -o /out/bin/osnadmin ./cmd/osnadmin \
    && cp sampleconfig/core.yaml sampleconfig/configtx.yaml sampleconfig/orderer.yaml /out/config/

FROM scratch

LABEL org.opencontainers.image.title="base/fabric-tools"
LABEL org.opencontainers.image.description="Hyperledger Fabric v2.5.16 tools rebuilt with the pinned Go toolchain"

COPY --from=build /out/bin/ /usr/local/bin/
COPY --from=build /out/config/ /etc/hyperledger/fabric/
