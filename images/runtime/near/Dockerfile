# 本文件定义 NEAR 本地网运行时镜像。
FROM nearprotocol/nearcore:2.2.1

LABEL org.opencontainers.image.title="runtime/near"
LABEL org.opencontainers.image.description="Chaimir NEAR localnet runtime"

USER root

# 创建非 root 用户和 NEAR 数据目录。
RUN groupadd --system chaimir \
    && useradd --system --gid chaimir --home-dir /workspace --create-home chaimir \
    && mkdir -p /runtime-state/near \
    && chown -R chaimir:chaimir /workspace /runtime-state

COPY entrypoint.sh /usr/local/bin/chaimir-near-entrypoint
RUN chmod +x /usr/local/bin/chaimir-near-entrypoint

USER chaimir
WORKDIR /workspace
EXPOSE 3030
ENTRYPOINT ["/usr/local/bin/chaimir-near-entrypoint"]
