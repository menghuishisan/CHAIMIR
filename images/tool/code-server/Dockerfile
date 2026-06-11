# 本文件定义 code-server 工具镜像,复用成熟 Web IDE 镜像。
FROM codercom/code-server:4.101.1

LABEL org.opencontainers.image.title="tool/code-server"
LABEL org.opencontainers.image.description="Chaimir code-server web IDE image"

USER root
RUN mkdir -p /workspace \
    && chown -R 1000:1000 /workspace

USER 1000:1000
WORKDIR /workspace
EXPOSE 8080

# code-server 只监听容器内端口,外部访问由平台代理鉴权。
CMD ["code-server", "--bind-addr", "0.0.0.0:8080", "--auth", "none", "/workspace"]
