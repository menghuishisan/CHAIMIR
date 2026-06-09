# 本文件定义长安链运行时镜像,复用官方 chainmaker 节点镜像能力。
FROM chainmakerofficial/chainmaker:v2.3.5

LABEL org.opencontainers.image.title="runtime/chainmaker"
LABEL org.opencontainers.image.description="Chaimir ChainMaker teaching runtime"

USER root

# 创建平台运行用户,链配置和状态由沙箱初始化卷提供。
RUN groupadd --system chaimir \
    && useradd --system --gid chaimir --home-dir /workspace --create-home chaimir \
    && mkdir -p /runtime-state/chainmaker \
    && chown -R chaimir:chaimir /workspace /runtime-state

COPY entrypoint.sh /usr/local/bin/chaimir-chainmaker-entrypoint
RUN chmod +x /usr/local/bin/chaimir-chainmaker-entrypoint

USER chaimir
WORKDIR /workspace
EXPOSE 12301
ENTRYPOINT ["/usr/local/bin/chaimir-chainmaker-entrypoint"]
