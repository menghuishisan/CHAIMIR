# 本文件定义 geth 私链运行时镜像,复用官方 go-ethereum 客户端。
FROM ethereum/client-go:v1.16.2

LABEL org.opencontainers.image.title="runtime/evm-geth"
LABEL org.opencontainers.image.description="Chaimir go-ethereum private-chain runtime"

USER root
RUN addgroup -S -g 1000 chaimir \
    && adduser -S -u 1000 -G chaimir -h /workspace chaimir \
    && mkdir -p /workspace /runtime-state/geth \
    && chown -R 1000:1000 /workspace /runtime-state

COPY entrypoint.sh /usr/local/bin/chaimir-geth-entrypoint
RUN chmod +x /usr/local/bin/chaimir-geth-entrypoint

USER 1000:1000
WORKDIR /workspace
EXPOSE 8545
ENTRYPOINT ["/usr/local/bin/chaimir-geth-entrypoint"]
