# 本文件定义 Aptos Move 本地网运行时镜像。
FROM aptoslabs/tools:aptos-node-v1.46.2

LABEL org.opencontainers.image.title="runtime/move-aptos"
LABEL org.opencontainers.image.description="Chaimir Aptos Move localnet runtime"

USER root

# 创建非 root 用户和 Aptos 本地网数据目录。
RUN apt-get -o Acquire::Retries=3 update \
    && apt-get -o Acquire::Retries=3 -y upgrade \
    && rm -rf /var/lib/apt/lists/* \
    && groupadd --gid 1000 chaimir \
    && useradd --uid 1000 --gid 1000 --home-dir /workspace --create-home --shell /usr/sbin/nologin chaimir \
    && mkdir -p /runtime-state/aptos \
    && chown -R 1000:1000 /workspace /runtime-state

COPY entrypoint.sh /usr/local/bin/chaimir-aptos-entrypoint
RUN chmod +x /usr/local/bin/chaimir-aptos-entrypoint

USER 1000:1000
WORKDIR /workspace
EXPOSE 8080
ENTRYPOINT ["/usr/local/bin/chaimir-aptos-entrypoint"]
