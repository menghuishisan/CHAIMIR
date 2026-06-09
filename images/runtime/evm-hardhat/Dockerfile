# 本文件定义 Hardhat 教学运行时镜像,用于 Solidity 开发体验类实验。
FROM node:20-bookworm-slim

LABEL org.opencontainers.image.title="runtime/evm-hardhat"
LABEL org.opencontainers.image.description="Chaimir Hardhat EVM teaching runtime"

USER root

# 安装基础依赖和 Foundry,Hardhat 作为全局工具固定主版本。
RUN apt-get update \
    && apt-get install -y --no-install-recommends ca-certificates curl git bash \
    && rm -rf /var/lib/apt/lists/* \
    && npm install -g hardhat@2.22.19 \
    && curl -L https://foundry.paradigm.xyz | bash \
    && /root/.foundry/bin/foundryup \
    && install -m 0755 /root/.foundry/bin/anvil /usr/local/bin/anvil \
    && install -m 0755 /root/.foundry/bin/forge /usr/local/bin/forge \
    && install -m 0755 /root/.foundry/bin/cast /usr/local/bin/cast \
    && groupadd --system chaimir \
    && useradd --system --gid chaimir --home-dir /workspace --create-home chaimir \
    && mkdir -p /runtime-state \
    && chown -R chaimir:chaimir /workspace /runtime-state

COPY entrypoint.sh /usr/local/bin/chaimir-hardhat-entrypoint
RUN chmod +x /usr/local/bin/chaimir-hardhat-entrypoint

USER chaimir
WORKDIR /workspace
EXPOSE 8545
ENTRYPOINT ["/usr/local/bin/chaimir-hardhat-entrypoint"]
