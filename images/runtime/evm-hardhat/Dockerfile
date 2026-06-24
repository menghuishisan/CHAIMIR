# 本文件定义 Hardhat 教学运行时镜像,用于 Solidity 开发体验类实验。
ARG NODE_BUILDER_IMAGE
FROM ghcr.io/foundry-rs/foundry:v1.0.0 AS foundry

FROM ${NODE_BUILDER_IMAGE}

LABEL org.opencontainers.image.title="runtime/evm-hardhat"
LABEL org.opencontainers.image.description="Chaimir Hardhat EVM teaching runtime"
ENV HOME=/workspace

USER root

# Foundry 工具来自固定上游镜像;这里只安装固定 Hardhat 版本并创建运行目录。
COPY --from=foundry /usr/local/bin/forge /usr/local/bin/forge
COPY --from=foundry /usr/local/bin/anvil /usr/local/bin/anvil
COPY --from=foundry /usr/local/bin/cast /usr/local/bin/cast
COPY --from=foundry /usr/local/bin/chisel /usr/local/bin/chisel
COPY package.json package-lock.json /opt/chaimir/evm-hardhat/
RUN apk upgrade --no-cache \
    && apk add --no-cache bash git ca-certificates \
    && cd /opt/chaimir/evm-hardhat \
    && npm ci --omit=dev --ignore-scripts --no-audit --no-fund \
    && ln -s /opt/chaimir/evm-hardhat/node_modules/.bin/hardhat /usr/local/bin/hardhat \
    && rm -rf /usr/local/lib/node_modules/npm /usr/local/lib/node_modules/corepack /usr/local/bin/npm /usr/local/bin/npx /usr/local/bin/corepack \
    && mkdir -p /workspace /runtime-state \
    && chown -R 1000:1000 /workspace /runtime-state

COPY entrypoint.sh /usr/local/bin/chaimir-hardhat-entrypoint
RUN chmod +x /usr/local/bin/chaimir-hardhat-entrypoint

USER 1000:1000
WORKDIR /workspace
EXPOSE 8545
ENTRYPOINT ["/usr/local/bin/chaimir-hardhat-entrypoint"]
