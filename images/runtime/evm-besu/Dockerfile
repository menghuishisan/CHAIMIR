# 本文件定义 Besu 企业 EVM 运行时镜像,复用 Hyperledger Besu 官方镜像。
FROM hyperledger/besu:24.12.2

LABEL org.opencontainers.image.title="runtime/evm-besu"
LABEL org.opencontainers.image.description="Chaimir Hyperledger Besu enterprise EVM runtime"

USER root

# 复用上游 besu:1000:1000,链状态只写入 runtime-state 卷。
RUN mkdir -p /workspace /runtime-state/besu \
    && chown -R 1000:1000 /workspace /runtime-state

COPY entrypoint.sh /usr/local/bin/chaimir-besu-entrypoint
RUN chmod +x /usr/local/bin/chaimir-besu-entrypoint

ENV HOME=/workspace
USER 1000:1000
WORKDIR /workspace
EXPOSE 8545
ENTRYPOINT ["/usr/local/bin/chaimir-besu-entrypoint"]
