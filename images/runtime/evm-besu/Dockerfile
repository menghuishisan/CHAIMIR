# 本文件定义 Besu 企业 EVM 运行时镜像,复用 Hyperledger Besu 官方镜像。
FROM hyperledger/besu:24.12.2

LABEL org.opencontainers.image.title="runtime/evm-besu"
LABEL org.opencontainers.image.description="Chaimir Hyperledger Besu enterprise EVM runtime"

USER root

# 创建运行目录并交给非 root 用户,链状态由 runtime-state 卷承载。
RUN groupadd --system chaimir \
    && useradd --system --gid chaimir --home-dir /workspace --create-home chaimir \
    && mkdir -p /runtime-state/besu \
    && chown -R chaimir:chaimir /workspace /runtime-state

COPY entrypoint.sh /usr/local/bin/chaimir-besu-entrypoint
RUN chmod +x /usr/local/bin/chaimir-besu-entrypoint

USER chaimir
WORKDIR /workspace
EXPOSE 8545
ENTRYPOINT ["/usr/local/bin/chaimir-besu-entrypoint"]
