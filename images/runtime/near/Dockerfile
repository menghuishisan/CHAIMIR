# 本文件定义 NEAR 本地网运行时镜像。
FROM nearprotocol/nearcore:2.2.1

LABEL org.opencontainers.image.title="runtime/near"
LABEL org.opencontainers.image.description="Chaimir NEAR localnet runtime"

USER root

# 创建非 root 用户和 NEAR 数据目录。
RUN apt-get -o Acquire::Retries=3 update \
    && apt-get -o Acquire::Retries=3 -y upgrade \
    && rm -rf /var/lib/apt/lists/* \
    && groupadd --gid 1000 chaimir \
    && useradd --uid 1000 --gid 1000 --home-dir /workspace --create-home --shell /usr/sbin/nologin chaimir \
    && mkdir -p /runtime-state/near \
    && chown -R 1000:1000 /workspace /runtime-state

COPY entrypoint.sh /usr/local/bin/chaimir-near-entrypoint
RUN chmod +x /usr/local/bin/chaimir-near-entrypoint

USER 1000:1000
WORKDIR /workspace
EXPOSE 3030
ENTRYPOINT ["/usr/local/bin/chaimir-near-entrypoint"]
