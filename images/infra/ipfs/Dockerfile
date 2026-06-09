# 本文件定义 IPFS/Kubo 节点镜像,复用官方 Kubo 镜像。
FROM ipfs/kubo:v0.34.1

LABEL org.opencontainers.image.title="infra/ipfs"
LABEL org.opencontainers.image.description="Chaimir IPFS Kubo node image"

USER root

# IPFS 数据目录由 runtime-state 卷承载,镜像内不写入业务数据。
RUN mkdir -p /data/ipfs /workspace \
    && chown -R ipfs:ipfs /data/ipfs /workspace

USER ipfs
WORKDIR /workspace
EXPOSE 5001 8080
CMD ["daemon", "--migrate=true"]
