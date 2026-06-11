# 本文件定义 Substrate/Polkadot 开发链运行时镜像。
FROM parity/substrate:v3.0.0

LABEL org.opencontainers.image.title="runtime/substrate"
LABEL org.opencontainers.image.description="Chaimir Substrate development chain runtime"

USER root

# 创建非 root 用户和链状态目录。
RUN groupadd --gid 1000 chaimir \
    && useradd --uid 1000 --gid 1000 --home-dir /workspace --create-home --shell /usr/sbin/nologin chaimir \
    && mkdir -p /runtime-state/substrate \
    && chown -R 1000:1000 /workspace /runtime-state

COPY entrypoint.sh /usr/local/bin/chaimir-substrate-entrypoint
RUN chmod +x /usr/local/bin/chaimir-substrate-entrypoint

USER 1000:1000
WORKDIR /workspace
EXPOSE 9944
ENTRYPOINT ["/usr/local/bin/chaimir-substrate-entrypoint"]
