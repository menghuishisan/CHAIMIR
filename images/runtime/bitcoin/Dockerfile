# 本文件定义 Bitcoin regtest 教学运行时镜像。
FROM ruimarinho/bitcoin-core:24-alpine

LABEL org.opencontainers.image.title="runtime/bitcoin"
LABEL org.opencontainers.image.description="Chaimir Bitcoin regtest runtime"

USER root

# 创建固定 UID/GID 的沙箱用户,避免 K8s 强制 1000:1000 后丢失 home 和目录权限。
RUN groupadd --gid 1000 chaimir \
    && useradd --uid 1000 --gid 1000 --home-dir /workspace --create-home --shell /usr/sbin/nologin chaimir \
    && mkdir -p /runtime-state/bitcoin \
    && chown -R 1000:1000 /workspace /runtime-state

COPY entrypoint.sh /usr/local/bin/chaimir-bitcoin-entrypoint
RUN chmod +x /usr/local/bin/chaimir-bitcoin-entrypoint

USER 1000:1000
WORKDIR /workspace
EXPOSE 18443
ENTRYPOINT ["/usr/local/bin/chaimir-bitcoin-entrypoint"]
