# 本文件定义 Fabric Explorer 工具镜像。
FROM hyperledger/explorer:1.1.8

LABEL org.opencontainers.image.title="tool/fabric-explorer"
LABEL org.opencontainers.image.description="Chaimir Hyperledger Fabric explorer image"

USER root
# Fabric Explorer 应从上游应用目录启动;工作区不是应用代码目录。
RUN mkdir -p /workspace /home/node/.npm \
    && chown -R 1000:1000 /workspace /home/node /opt/explorer

USER 1000:1000
WORKDIR /opt/explorer
EXPOSE 8080
