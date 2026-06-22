# 本文件定义 Ganache 轻量 EVM 教学运行时镜像。
FROM node:20-bookworm-slim

LABEL org.opencontainers.image.title="runtime/evm-ganache"
LABEL org.opencontainers.image.description="Chaimir Ganache lightweight EVM runtime"

ARG GANACHE_VERSION=7.9.2
ENV HOME=/workspace

COPY package.json package-lock.json /opt/chaimir/evm-ganache/
RUN apt-get -o Acquire::Retries=3 update \
    && apt-get -o Acquire::Retries=3 -y upgrade \
    && rm -rf /var/lib/apt/lists/* \
    && cd /opt/chaimir/evm-ganache \
    && npm ci --omit=dev --ignore-scripts --no-audit --no-fund \
    && cd /opt/chaimir/evm-ganache/node_modules/ganache \
    && npm install --omit=dev --ignore-scripts --no-audit --no-fund --package-lock=false elliptic@6.6.1 lodash@4.18.0 secp256k1@4.0.4 \
    && mkdir -p /tmp/chaimir-npm-ws \
    && cd /tmp/chaimir-npm-ws \
    && npm install --omit=dev --ignore-scripts --no-audit --no-fund --package-lock=false ws@8.21.0 \
    && rm -rf /opt/chaimir/evm-ganache/node_modules/ganache/node_modules/@trufflesuite/uws-js-unofficial/node_modules/ws \
    && cp -a /tmp/chaimir-npm-ws/node_modules/ws /opt/chaimir/evm-ganache/node_modules/ganache/node_modules/@trufflesuite/uws-js-unofficial/node_modules/ws \
    && rm -rf /tmp/chaimir-npm-ws \
    && ln -s /opt/chaimir/evm-ganache/node_modules/.bin/ganache /usr/local/bin/ganache \
    && rm -rf /workspace/.npm /usr/local/lib/node_modules/npm /usr/local/lib/node_modules/corepack /usr/local/bin/npm /usr/local/bin/npx /usr/local/bin/corepack \
    && mkdir -p /workspace /runtime-state \
    && chown -R 1000:1000 /workspace /runtime-state

COPY entrypoint.sh /usr/local/bin/chaimir-ganache-entrypoint
RUN chmod +x /usr/local/bin/chaimir-ganache-entrypoint

USER 1000:1000
WORKDIR /workspace
EXPOSE 8545
ENTRYPOINT ["/usr/local/bin/chaimir-ganache-entrypoint"]
