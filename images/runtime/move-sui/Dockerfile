# 本文件定义 Sui Move 本地网运行时镜像。
FROM mysten/sui-tools:086c222823a48f5136eedf969e9d489dc265e714

LABEL org.opencontainers.image.title="runtime/move-sui"
LABEL org.opencontainers.image.description="Chaimir Sui Move localnet runtime"

USER root

# 创建非 root 用户和 Sui 数据目录。
RUN apt-get -o Acquire::Retries=3 update \
    && apt-get -o Acquire::Retries=3 -y upgrade \
    && rm -rf /var/lib/apt/lists/* \
    && groupadd --gid 1000 chaimir \
    && useradd --uid 1000 --gid 1000 --home-dir /workspace --create-home --shell /usr/sbin/nologin chaimir \
    && mkdir -p /runtime-state/sui \
    && chown -R 1000:1000 /workspace /runtime-state

COPY entrypoint.sh /usr/local/bin/chaimir-sui-entrypoint
RUN chmod +x /usr/local/bin/chaimir-sui-entrypoint

USER 1000:1000
WORKDIR /workspace
EXPOSE 9000
ENTRYPOINT ["/usr/local/bin/chaimir-sui-entrypoint"]
