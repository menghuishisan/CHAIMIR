# 本文件定义 FISCO BCOS 运行时镜像,复用官方节点镜像能力。
FROM fiscoorg/fiscobcos:v3.12.1

LABEL org.opencontainers.image.title="runtime/fisco-bcos"
LABEL org.opencontainers.image.description="Chaimir FISCO BCOS teaching runtime"

USER root

# 建立非 root 工作区和链状态目录,组网细节由 M2 runtime manifest 编排。
RUN groupadd --gid 1000 chaimir \
    && useradd --uid 1000 --gid 1000 --home-dir /workspace --create-home --shell /usr/sbin/nologin chaimir \
    && mkdir -p /runtime-state/fisco \
    && chown -R 1000:1000 /workspace /runtime-state

COPY entrypoint.sh /usr/local/bin/chaimir-fisco-entrypoint
RUN chmod +x /usr/local/bin/chaimir-fisco-entrypoint

USER 1000:1000
WORKDIR /workspace
EXPOSE 20200 8545
ENTRYPOINT ["/usr/local/bin/chaimir-fisco-entrypoint"]
