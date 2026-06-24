# 本文件定义 Solana 本地测试链运行时镜像。
FROM solanalabs/solana:v1.18.26

LABEL org.opencontainers.image.title="runtime/solana"
LABEL org.opencontainers.image.description="Chaimir Solana test validator runtime"

USER root

# 创建固定 UID/GID 的沙箱用户,测试链账本只写入 runtime-state。
RUN apt-get -o Acquire::Retries=3 update \
    && apt-get -o Acquire::Retries=3 -y upgrade \
    && rm -rf /var/lib/apt/lists/* \
    && groupadd --gid 1000 chaimir \
    && useradd --uid 1000 --gid 1000 --home-dir /workspace --create-home --shell /usr/sbin/nologin chaimir \
    && mkdir -p /runtime-state/solana \
    && chown -R 1000:1000 /workspace /runtime-state

COPY entrypoint.sh /usr/local/bin/chaimir-solana-entrypoint
RUN chmod +x /usr/local/bin/chaimir-solana-entrypoint

USER 1000:1000
WORKDIR /workspace
EXPOSE 8899
ENTRYPOINT ["/usr/local/bin/chaimir-solana-entrypoint"]
