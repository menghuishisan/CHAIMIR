# 本文件定义长安链运行时镜像,复用官方 chainmaker 节点镜像能力。
FROM chainmakerofficial/chainmaker:v2.3.5

LABEL org.opencontainers.image.title="runtime/chainmaker"
LABEL org.opencontainers.image.description="Chaimir ChainMaker teaching runtime"

USER root

# 创建固定 UID/GID 的沙箱用户,链配置和状态由沙箱初始化卷提供。
RUN groupadd --gid 1000 chaimir \
    && useradd --uid 1000 --gid 1000 --home-dir /workspace --create-home --shell /usr/sbin/nologin chaimir \
    && mkdir -p /runtime-state/chainmaker \
    && chown -R 1000:1000 /workspace /runtime-state

COPY entrypoint.sh /usr/local/bin/chaimir-chainmaker-entrypoint
RUN chmod +x /usr/local/bin/chaimir-chainmaker-entrypoint

USER 1000:1000
WORKDIR /workspace
EXPOSE 12301
ENTRYPOINT ["/usr/local/bin/chaimir-chainmaker-entrypoint"]
