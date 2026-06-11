# 本文件定义 Remix Solidity IDE 工具镜像。
FROM remixproject/remix-ide:remix_live

LABEL org.opencontainers.image.title="tool/remix"
LABEL org.opencontainers.image.description="Chaimir Remix Solidity IDE image"

USER root

# 创建工作区目录,Remix 静态服务由上游镜像提供。
RUN mkdir -p /workspace \
    && chown -R 1000:1000 /workspace

USER 1000:1000
WORKDIR /workspace
EXPOSE 8080
