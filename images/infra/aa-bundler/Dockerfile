# 本文件定义 ERC-4337 账户抽象 bundler 镜像,复用 Stackup bundler 上游镜像。
FROM ghcr.io/stackup-wallet/stackup-bundler:v0.6.25

LABEL org.opencontainers.image.title="infra/aa-bundler"
LABEL org.opencontainers.image.description="Chaimir ERC-4337 bundler image"

USER root

RUN mkdir -p /workspace /bundler \
    && chown -R 1000:1000 /workspace /bundler

COPY entrypoint.sh /usr/local/bin/chaimir-aa-bundler-entrypoint
RUN chmod +x /usr/local/bin/chaimir-aa-bundler-entrypoint

USER 1000
WORKDIR /workspace
EXPOSE 4337
ENTRYPOINT ["/usr/local/bin/chaimir-aa-bundler-entrypoint"]
