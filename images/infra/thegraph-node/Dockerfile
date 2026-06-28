# 本文件定义 The Graph 索引节点镜像,复用官方 graph-node 二进制并收紧运行层。
FROM graphprotocol/graph-node:latest AS upstream

FROM cgr.dev/chainguard/wolfi-base:latest

LABEL org.opencontainers.image.title="infra/thegraph-node"
LABEL org.opencontainers.image.description="Chaimir The Graph indexing node image"

COPY --from=upstream /usr/local/bin/graph-node /usr/local/bin/graph-node
RUN apk add --no-cache ca-certificates glibc libgcc \
    && addgroup -g 1000 chaimir \
    && adduser -D -u 1000 -G chaimir -h /workspace -s /sbin/nologin chaimir \
    && chown 1000:1000 /workspace \
    && test -x /usr/local/bin/graph-node

USER 1000:1000
WORKDIR /workspace
EXPOSE 8000 8020 8030 8040
CMD ["graph-node"]
