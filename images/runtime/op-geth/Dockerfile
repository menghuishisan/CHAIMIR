# OP Geth platform adapter: rebuild the pinned upstream commit with the
# current platform toolchain, then initialize a supplied rollup genesis and
# expose authenticated Engine API plus JSON-RPC with fixed defaults.
ARG GO_BUILDER_IMAGE=invalid.invalid/base/go-builder:required

FROM ${GO_BUILDER_IMAGE} AS builder

ARG OP_GETH_COMMIT=e8800cffe53d459cde8a07c8e8f1de9d86e79e07
ARG OP_GETH_SOURCE_SHA256=82001bf36be8425dddb7c0c3f231b3dd8572e40576e4148d323f9146d7181191
ARG OP_GETH_SOURCE_URL=https://codeload.github.com/ethereum-optimism/op-geth/tar.gz/e8800cffe53d459cde8a07c8e8f1de9d86e79e07
ARG GO_MODULE_PROXY=

USER root
RUN apk add --no-cache tar
WORKDIR /src/op-geth
ADD ${OP_GETH_SOURCE_URL} /tmp/op-geth-source.tar.gz
RUN sha256sum /tmp/op-geth-source.tar.gz \
    && echo "${OP_GETH_SOURCE_SHA256}  /tmp/op-geth-source.tar.gz" | sha256sum -c - \
    && tar -xzf /tmp/op-geth-source.tar.gz --strip-components=1 -C /src/op-geth \
    && rm -f /tmp/op-geth-source.tar.gz \
    && chown -R 1000:1000 /src/op-geth

USER 1000:1000
RUN if [ -n "${GO_MODULE_PROXY}" ]; then go env -w GOPROXY="${GO_MODULE_PROXY}"; fi \
    && go get \
        go.opentelemetry.io/otel@v1.43.0 \
        go.opentelemetry.io/otel/sdk@v1.43.0 \
        golang.org/x/crypto@v0.53.0 \
        golang.org/x/net@v0.56.0 \
        golang.org/x/text@v0.39.0 \
        google.golang.org/grpc@v1.82.1 \
    && go mod tidy \
    && CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go run build/ci.go install -static ./cmd/geth

COPY entrypoint.go /src/entrypoint.go
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath -buildvcs=false -ldflags="-s -w" -o /out/chaimir-op-geth-entrypoint /src/entrypoint.go

FROM alpine:3.23@sha256:fd791d74b68913cbb027c6546007b3f0d3bc45125f797758156952bc2d6daf40

LABEL org.opencontainers.image.title="runtime/op-geth"
LABEL org.opencontainers.image.description="Chaimir OP Geth source rebuild with runtime adapter"

RUN apk upgrade --no-cache \
    && apk add --no-cache ca-certificates curl \
    && mkdir -p /runtime-state/op-geth \
    && chown -R 1000:1000 /runtime-state

COPY --from=builder --chown=1000:1000 /src/op-geth/build/bin/geth /usr/local/bin/geth
COPY --from=builder --chown=1000:1000 /out/chaimir-op-geth-entrypoint /usr/local/bin/chaimir-op-geth-entrypoint
USER 1000:1000
WORKDIR /workspace
EXPOSE 8545 8551
ENTRYPOINT ["/usr/local/bin/chaimir-op-geth-entrypoint"]
