# 本文件定义 Foundry/Anvil 运行时镜像,复用成熟官方 Foundry 镜像能力。
FROM ghcr.io/foundry-rs/foundry:v1.0.0

LABEL org.opencontainers.image.title="runtime/evm-foundry"
LABEL org.opencontainers.image.description="Chaimir Foundry and Anvil EVM runtime"

USER root

# 创建非 root 用户和工作区,运行时由平台挂载 workspace/runtime-state。
RUN addgroup -S chaimir \
    && adduser -S -G chaimir -h /workspace chaimir \
    && mkdir -p /workspace /runtime-state \
    && chown -R chaimir:chaimir /workspace /runtime-state

COPY entrypoint.sh /usr/local/bin/chaimir-foundry-entrypoint
RUN chmod +x /usr/local/bin/chaimir-foundry-entrypoint

USER chaimir
WORKDIR /workspace
EXPOSE 8545
ENTRYPOINT ["/usr/local/bin/chaimir-foundry-entrypoint"]
