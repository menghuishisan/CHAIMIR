# 本文件定义 RPC 网关镜像,复用 Envoy 作为生产级反向代理。
ARG GO_BUILDER_IMAGE=invalid.invalid/base/go-builder:required

FROM ${GO_BUILDER_IMAGE} AS entrypoint-build
WORKDIR /src/rpc-gateway
COPY entrypoint.go envoy.yaml ./
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath -ldflags="-s -w" -o /out/chaimir-rpc-gateway-entrypoint ./entrypoint.go

FROM envoyproxy/envoy:v1.32.3@sha256:127e33c48c60be9a148cf9256d52e734ceda28aa344145023933116c7e524beb

LABEL org.opencontainers.image.title="infra/rpc-gateway"
LABEL org.opencontainers.image.description="Chaimir RPC gateway proxy image"

USER root

RUN mkdir -p /workspace && chown 1000:1000 /workspace
COPY --chmod=0755 --from=entrypoint-build /out/chaimir-rpc-gateway-entrypoint /usr/local/bin/chaimir-rpc-gateway-entrypoint

USER 1000:1000
WORKDIR /workspace
EXPOSE 8080
ENTRYPOINT ["/usr/local/bin/chaimir-rpc-gateway-entrypoint"]
