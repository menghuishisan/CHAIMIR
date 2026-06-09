# 本文件定义 Ganache 轻量 EVM 教学运行时镜像。
FROM node:20-bookworm-slim

LABEL org.opencontainers.image.title="runtime/evm-ganache"
LABEL org.opencontainers.image.description="Chaimir Ganache lightweight EVM runtime"

ARG GANACHE_VERSION=7.9.2

COPY package.json package-lock.json /opt/chaimir/evm-ganache/
RUN groupadd --system chaimir \
    && useradd --system --gid chaimir --home-dir /workspace --create-home chaimir \
    && cd /opt/chaimir/evm-ganache \
    && npm ci --omit=dev --ignore-scripts --no-audit --no-fund \
    && ln -s /opt/chaimir/evm-ganache/node_modules/.bin/ganache /usr/local/bin/ganache \
    && mkdir -p /runtime-state \
    && chown -R chaimir:chaimir /workspace /runtime-state

COPY entrypoint.sh /usr/local/bin/chaimir-ganache-entrypoint
RUN chmod +x /usr/local/bin/chaimir-ganache-entrypoint

USER chaimir
WORKDIR /workspace
EXPOSE 8545
ENTRYPOINT ["/usr/local/bin/chaimir-ganache-entrypoint"]
