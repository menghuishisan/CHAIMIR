# 本文件定义 Hardhat 教学运行时镜像,用于 Solidity 开发体验类实验。
FROM ghcr.io/foundry-rs/foundry:v1.0.0

LABEL org.opencontainers.image.title="runtime/evm-hardhat"
LABEL org.opencontainers.image.description="Chaimir Hardhat EVM teaching runtime"

USER root

# Foundry 工具来自固定上游镜像;这里只安装固定 Hardhat 版本并创建运行目录。
COPY package.json package-lock.json /opt/chaimir/evm-hardhat/
RUN apk add --no-cache nodejs npm bash git \
    && cd /opt/chaimir/evm-hardhat \
    && npm ci --omit=dev --ignore-scripts --no-audit --no-fund \
    && ln -s /opt/chaimir/evm-hardhat/node_modules/.bin/hardhat /usr/local/bin/hardhat \
    && addgroup -S chaimir \
    && adduser -S -G chaimir -h /workspace chaimir \
    && mkdir -p /runtime-state \
    && chown -R chaimir:chaimir /workspace /runtime-state

COPY entrypoint.sh /usr/local/bin/chaimir-hardhat-entrypoint
RUN chmod +x /usr/local/bin/chaimir-hardhat-entrypoint

USER chaimir
WORKDIR /workspace
EXPOSE 8545
ENTRYPOINT ["/usr/local/bin/chaimir-hardhat-entrypoint"]
