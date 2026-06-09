# 本文件定义 OP Stack 节点工具镜像,复用 Optimism op-node 上游镜像。
FROM us-docker.pkg.dev/oplabs-tools-artifacts/images/op-node:v1.10.2

LABEL org.opencontainers.image.title="infra/op-stack"
LABEL org.opencontainers.image.description="Chaimir OP Stack node tool image"

USER root

RUN mkdir -p /workspace /op-stack \
    && chown -R 1000:1000 /workspace /op-stack

COPY entrypoint.sh /usr/local/bin/chaimir-op-stack-entrypoint
RUN chmod +x /usr/local/bin/chaimir-op-stack-entrypoint

USER 1000
WORKDIR /workspace
EXPOSE 8545 9003
ENTRYPOINT ["/usr/local/bin/chaimir-op-stack-entrypoint"]
