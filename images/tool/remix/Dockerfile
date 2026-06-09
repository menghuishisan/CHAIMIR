# 本文件定义 Remix Solidity IDE 工具镜像。
FROM remixproject/remix-ide:2.0.0

LABEL org.opencontainers.image.title="tool/remix"
LABEL org.opencontainers.image.description="Chaimir Remix Solidity IDE image"

USER root

# 创建工作区目录,Remix 静态服务由上游镜像提供。
RUN mkdir -p /workspace \
    && chown -R 101:101 /workspace

USER 101
WORKDIR /workspace
EXPOSE 8080
