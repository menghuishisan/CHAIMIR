# 本文件定义 Fabric 工具运行时镜像,复用 Hyperledger Fabric 官方 tools 镜像。
FROM hyperledger/fabric-tools:2.5

LABEL org.opencontainers.image.title="runtime/fabric"
LABEL org.opencontainers.image.description="Chaimir Hyperledger Fabric runtime tools image"

USER root

# 创建平台工作区用户;Fabric 官方工具仍由镜像提供,不在这里自研。
RUN groupadd --system chaimir \
    && useradd --system --gid chaimir --home-dir /workspace --create-home chaimir \
    && mkdir -p /runtime-state \
    && chown -R chaimir:chaimir /workspace /runtime-state

COPY entrypoint.sh /usr/local/bin/chaimir-fabric-entrypoint
RUN chmod +x /usr/local/bin/chaimir-fabric-entrypoint

USER chaimir
WORKDIR /workspace
ENTRYPOINT ["/usr/local/bin/chaimir-fabric-entrypoint"]
