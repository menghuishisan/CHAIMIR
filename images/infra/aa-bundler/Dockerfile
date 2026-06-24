# 本文件定义 ERC-4337 账户抽象 bundler 镜像,复用仍维护的 Rundler 上游镜像。
FROM alchemyplatform/rundler:v0.11.0@sha256:ffdc5fca2d5566152af6fe96f88c50a4cf22ce88852bb626c591d3e8ea426178

LABEL org.opencontainers.image.title="infra/aa-bundler"
LABEL org.opencontainers.image.description="Chaimir ERC-4337 bundler image"

USER root

RUN apt-get update \
    && apt-get upgrade -y --no-install-recommends \
    && rm -rf /var/lib/apt/lists/* \
    && mkdir -p /workspace /bundler \
    && chown -R 1000:1000 /workspace /bundler

COPY entrypoint.sh /usr/local/bin/chaimir-aa-bundler-entrypoint
RUN chmod +x /usr/local/bin/chaimir-aa-bundler-entrypoint

USER 1000:1000
WORKDIR /workspace
EXPOSE 3000 8080
ENTRYPOINT ["/usr/local/bin/chaimir-aa-bundler-entrypoint"]
