# 本文件定义 Blockscout 区块链浏览器工具镜像。
FROM blockscout/blockscout:6.9.2

LABEL org.opencontainers.image.title="tool/blockscout"
LABEL org.opencontainers.image.description="Chaimir Blockscout EVM explorer image"

USER root
RUN mkdir -p /workspace \
    && chown -R 1000:1000 /workspace

USER 1000
WORKDIR /workspace
EXPOSE 4000
