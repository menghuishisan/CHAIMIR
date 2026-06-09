# 本文件定义 Bitcoin regtest 教学运行时镜像。
FROM ruimarinho/bitcoin-core:26

LABEL org.opencontainers.image.title="runtime/bitcoin"
LABEL org.opencontainers.image.description="Chaimir Bitcoin regtest runtime"

USER root

# 创建非 root 用户和 bitcoind 数据目录。
RUN groupadd --system chaimir \
    && useradd --system --gid chaimir --home-dir /workspace --create-home chaimir \
    && mkdir -p /runtime-state/bitcoin \
    && chown -R chaimir:chaimir /workspace /runtime-state

COPY entrypoint.sh /usr/local/bin/chaimir-bitcoin-entrypoint
RUN chmod +x /usr/local/bin/chaimir-bitcoin-entrypoint

USER chaimir
WORKDIR /workspace
EXPOSE 18443
ENTRYPOINT ["/usr/local/bin/chaimir-bitcoin-entrypoint"]
