# 本文件定义 Hardhat 教学运行时镜像,用于 Solidity 开发体验类实验。
FROM ghcr.io/foundry-rs/foundry:v1.7.1@sha256:bdc584033cefab8be282c7f57c263cda42f04700cd1ec88d64bebe2f645aba72 AS foundry

FROM node:22-trixie-slim@sha256:a05717adfe7289e2a0fa36a694dc430a510adab6467c7036e51551198935abef

LABEL org.opencontainers.image.title="runtime/evm-hardhat"
LABEL org.opencontainers.image.description="Chaimir Hardhat EVM teaching runtime"
ENV HOME=/workspace
ARG SOLC_0_8_20_SHA256=0479d44fdf9c501c25337fdc540419f1593b884a87b47f023da4f1c700fda782
ARG NPM_REGISTRY=

USER root

# Foundry 工具来自固定上游镜像;这里只安装固定 Hardhat 版本并创建运行目录。
COPY --from=foundry /usr/local/bin/forge /usr/local/bin/forge
COPY --from=foundry /usr/local/bin/anvil /usr/local/bin/anvil
COPY --from=foundry /usr/local/bin/cast /usr/local/bin/cast
COPY --from=foundry /usr/local/bin/chisel /usr/local/bin/chisel
COPY package.json package-lock.json /opt/chaimir/evm-hardhat/
ADD --checksum=sha256:0479d44fdf9c501c25337fdc540419f1593b884a87b47f023da4f1c700fda782 https://github.com/ethereum/solidity/releases/download/v0.8.20/solc-static-linux /tmp/solc-0.8.20
RUN sed -i 's|http://deb.debian.org/debian$|http://ftp.de.debian.org/debian|' /etc/apt/sources.list.d/debian.sources \
    && apt-get -o Acquire::Retries=8 update \
    && DEBIAN_FRONTEND=noninteractive apt-get -o Acquire::Retries=5 dist-upgrade -y --no-install-recommends \
    && DEBIAN_FRONTEND=noninteractive apt-get -o Acquire::Retries=5 install -y --no-install-recommends bash git ca-certificates jq \
    && rm -rf /var/lib/apt/lists/* \
    && echo "${SOLC_0_8_20_SHA256}  /tmp/solc-0.8.20" | sha256sum -c - \
    && install -m 0755 /tmp/solc-0.8.20 /usr/local/bin/solc-0.8.20 \
    && ln -sf /usr/local/bin/solc-0.8.20 /usr/local/bin/solc \
    && rm -f /tmp/solc-0.8.20 \
    && cd /opt/chaimir/evm-hardhat \
    && if [ -n "${NPM_REGISTRY}" ]; then npm config set registry "${NPM_REGISTRY}"; fi \
    && npm ci --omit=dev --ignore-scripts --no-audit --no-fund \
    && ln -s /opt/chaimir/evm-hardhat/node_modules/.bin/hardhat /usr/local/bin/hardhat \
    && mkdir -p /workspace /runtime-state \
    && chown -R 1000:1000 /workspace /runtime-state

COPY entrypoint.sh /usr/local/bin/chaimir-hardhat-entrypoint
COPY chaimir-hardhat-adapter /usr/local/bin/chaimir-hardhat-adapter
RUN chmod +x /usr/local/bin/chaimir-hardhat-entrypoint /usr/local/bin/chaimir-hardhat-adapter

USER 1000:1000
WORKDIR /workspace
EXPOSE 8545
ENTRYPOINT ["/usr/local/bin/chaimir-hardhat-entrypoint"]
