# 本文件定义 Sui Move 本地网运行时镜像。
FROM mysten/sui-tools:v1.28.3

LABEL org.opencontainers.image.title="runtime/move-sui"
LABEL org.opencontainers.image.description="Chaimir Sui Move localnet runtime"

USER root

# 创建非 root 用户和 Sui 数据目录。
RUN groupadd --system chaimir \
    && useradd --system --gid chaimir --home-dir /workspace --create-home chaimir \
    && mkdir -p /runtime-state/sui \
    && chown -R chaimir:chaimir /workspace /runtime-state

COPY entrypoint.sh /usr/local/bin/chaimir-sui-entrypoint
RUN chmod +x /usr/local/bin/chaimir-sui-entrypoint

USER chaimir
WORKDIR /workspace
EXPOSE 9000
ENTRYPOINT ["/usr/local/bin/chaimir-sui-entrypoint"]
