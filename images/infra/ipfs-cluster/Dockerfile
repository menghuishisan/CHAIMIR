# 本文件定义 IPFS Cluster 镜像,用于多节点存储实验。
FROM ipfs/ipfs-cluster:v1.1.3

LABEL org.opencontainers.image.title="infra/ipfs-cluster"
LABEL org.opencontainers.image.description="Chaimir IPFS Cluster coordination image"

USER root

# 集群配置和密钥由运行期注入,镜像只准备非 root 运行目录。
RUN mkdir -p /data/ipfs-cluster /workspace \
    && chown -R 1000:1000 /data/ipfs-cluster /workspace

USER 1000:1000
WORKDIR /workspace
EXPOSE 9094 9096
CMD ["daemon"]
