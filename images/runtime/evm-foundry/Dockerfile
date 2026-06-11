# 本文件定义 Foundry/Anvil 运行时镜像,复用成熟官方 Foundry 镜像能力。
FROM ghcr.io/foundry-rs/foundry:v1.0.0

LABEL org.opencontainers.image.title="runtime/evm-foundry"
LABEL org.opencontainers.image.description="Chaimir Foundry and Anvil EVM runtime"

USER root

# 复用上游 foundry:1000:1000,缓存写入 runtime-state 而不是学生 workspace。
RUN mkdir -p /workspace /runtime-state/foundry-home \
    && chown -R 1000:1000 /workspace /runtime-state

COPY entrypoint.sh /usr/local/bin/chaimir-foundry-entrypoint
RUN chmod +x /usr/local/bin/chaimir-foundry-entrypoint

ENV HOME=/runtime-state/foundry-home
USER 1000:1000
WORKDIR /workspace
EXPOSE 8545
ENTRYPOINT ["/usr/local/bin/chaimir-foundry-entrypoint"]
