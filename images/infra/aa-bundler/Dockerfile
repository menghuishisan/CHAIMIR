# 本文件定义 ERC-4337 账户抽象 bundler 镜像,复用 Stackup bundler 上游镜像。
FROM ghcr.io/stackup-wallet/stackup-bundler:v0.6.25

LABEL org.opencontainers.image.title="infra/aa-bundler"
LABEL org.opencontainers.image.description="Chaimir ERC-4337 bundler image"

USER root

RUN mkdir -p /workspace /bundler \
    && chown -R 1000:1000 /workspace /bundler

USER 1000
WORKDIR /workspace
EXPOSE 4337
CMD ["stackup-bundler", "--help"]
