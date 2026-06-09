# 本文件定义 FISCO BCOS 运行时镜像,复用官方节点镜像能力。
FROM fiscoorg/fiscobcos:v3.11.0

LABEL org.opencontainers.image.title="runtime/fisco-bcos"
LABEL org.opencontainers.image.description="Chaimir FISCO BCOS teaching runtime"

USER root

# 建立非 root 工作区和链状态目录,组网细节由 M2 runtime manifest 编排。
RUN groupadd --system chaimir \
    && useradd --system --gid chaimir --home-dir /workspace --create-home chaimir \
    && mkdir -p /runtime-state/fisco \
    && chown -R chaimir:chaimir /workspace /runtime-state

COPY entrypoint.sh /usr/local/bin/chaimir-fisco-entrypoint
RUN chmod +x /usr/local/bin/chaimir-fisco-entrypoint

USER chaimir
WORKDIR /workspace
EXPOSE 20200 8545
ENTRYPOINT ["/usr/local/bin/chaimir-fisco-entrypoint"]
