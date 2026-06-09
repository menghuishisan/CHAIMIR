# 本文件定义 Fabric Explorer 工具镜像。
FROM hyperledger/explorer:1.1.8

LABEL org.opencontainers.image.title="tool/fabric-explorer"
LABEL org.opencontainers.image.description="Chaimir Hyperledger Fabric explorer image"

USER root
RUN mkdir -p /workspace \
    && chown -R 1000:1000 /workspace

USER 1000
WORKDIR /workspace
EXPOSE 8080
