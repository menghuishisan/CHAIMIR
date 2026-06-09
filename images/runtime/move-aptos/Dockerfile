# 本文件定义 Aptos Move 本地网运行时镜像。
FROM aptoslabs/tools:aptos-node-v1.27.3

LABEL org.opencontainers.image.title="runtime/move-aptos"
LABEL org.opencontainers.image.description="Chaimir Aptos Move localnet runtime"

USER root

# 创建非 root 用户和 Aptos 本地网数据目录。
RUN groupadd --system chaimir \
    && useradd --system --gid chaimir --home-dir /workspace --create-home chaimir \
    && mkdir -p /runtime-state/aptos \
    && chown -R chaimir:chaimir /workspace /runtime-state

COPY entrypoint.sh /usr/local/bin/chaimir-aptos-entrypoint
RUN chmod +x /usr/local/bin/chaimir-aptos-entrypoint

USER chaimir
WORKDIR /workspace
EXPOSE 8080
ENTRYPOINT ["/usr/local/bin/chaimir-aptos-entrypoint"]
