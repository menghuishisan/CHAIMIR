# 本文件从共享 Fabric 工具基座构建原生 CLI 浏览器服务,不引入已归档的旧上游应用。
ARG FABRIC_TOOLS_IMAGE=invalid.invalid/base/fabric-tools:required
ARG GO_BUILDER_IMAGE=invalid.invalid/base/go-builder:required
FROM ${FABRIC_TOOLS_IMAGE} AS fabric-tools
FROM ${GO_BUILDER_IMAGE} AS build
WORKDIR /src
COPY server.go .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath -ldflags="-s -w" -o /out/chaimir-fabric-explorer ./server.go

FROM alpine:3.23@sha256:fd791d74b68913cbb027c6546007b3f0d3bc45125f797758156952bc2d6daf40

LABEL org.opencontainers.image.title="tool/fabric-explorer"
LABEL org.opencontainers.image.description="Chaimir native Hyperledger Fabric explorer"

USER root
RUN apk add --no-cache \
        ca-certificates \
        'libcrypto3>=3.5.8-r0' \
        'libssl3>=3.5.8-r0' \
    && addgroup -S -g 1000 chaimir \
    && adduser -S -u 1000 -G chaimir -h /workspace -s /sbin/nologin chaimir \
    && mkdir -p /workspace /tmp \
    && chown -R 1000:1000 /workspace /tmp
COPY --from=fabric-tools /usr/local/bin/peer /usr/local/bin/peer
COPY --from=fabric-tools /etc/hyperledger/fabric/ /etc/hyperledger/fabric/
COPY --from=build /out/chaimir-fabric-explorer /usr/local/bin/chaimir-fabric-explorer
RUN chmod 0755 /usr/local/bin/peer /usr/local/bin/chaimir-fabric-explorer

ENV FABRIC_CFG_PATH=/etc/hyperledger/fabric
USER 1000:1000
WORKDIR /workspace
EXPOSE 8080
ENTRYPOINT ["/usr/local/bin/chaimir-fabric-explorer"]
