# 本文件定义 Cosmos SDK/Tendermint 单链运行时镜像。
FROM ghcr.io/cosmos/gaia:v18.1.0

LABEL org.opencontainers.image.title="runtime/cosmos"
LABEL org.opencontainers.image.description="Chaimir Cosmos SDK local chain runtime"

USER root

# 创建非 root 用户和 Cosmos 数据目录。
RUN groupadd --system chaimir \
    && useradd --system --gid chaimir --home-dir /workspace --create-home chaimir \
    && mkdir -p /runtime-state/cosmos \
    && chown -R chaimir:chaimir /workspace /runtime-state

COPY entrypoint.sh /usr/local/bin/chaimir-cosmos-entrypoint
RUN chmod +x /usr/local/bin/chaimir-cosmos-entrypoint

USER chaimir
WORKDIR /workspace
EXPOSE 26657 1317
ENTRYPOINT ["/usr/local/bin/chaimir-cosmos-entrypoint"]
