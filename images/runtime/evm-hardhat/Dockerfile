# 本文件定义 Hardhat 教学运行时镜像,用于 Solidity 开发体验类实验。
FROM ghcr.io/foundry-rs/foundry:v1.7.1@sha256:bdc584033cefab8be282c7f57c263cda42f04700cd1ec88d64bebe2f645aba72 AS foundry

FROM node:22-bookworm-slim@sha256:83f487e0a63425e5b4d146fb5e5be574bcbe1b7b843d3ebafdd95eaf7767a7e5 AS build

LABEL org.opencontainers.image.title="runtime/evm-hardhat"
LABEL org.opencontainers.image.description="Chaimir Hardhat EVM teaching runtime"
ENV HOME=/workspace
ARG SOLC_0_8_20_SHA256=0479d44fdf9c501c25337fdc540419f1593b884a87b47f023da4f1c700fda782
ARG JQ_SHA256=5942c9b0934e510ee61eb3e30273f1b3fe2590df93933a93d7c58b81d19c8ff5
ARG NPM_REGISTRY=https://registry.npmmirror.com

USER root

# Foundry 工具来自固定上游镜像;这里只安装固定 Hardhat 版本并创建运行目录。
COPY --from=foundry /usr/local/bin/forge /usr/local/bin/forge
COPY --from=foundry /usr/local/bin/anvil /usr/local/bin/anvil
COPY --from=foundry /usr/local/bin/cast /usr/local/bin/cast
COPY --from=foundry /usr/local/bin/chisel /usr/local/bin/chisel
COPY package.json package-lock.json /opt/chaimir/evm-hardhat/
ADD --checksum=sha256:0479d44fdf9c501c25337fdc540419f1593b884a87b47f023da4f1c700fda782 https://github.com/ethereum/solidity/releases/download/v0.8.20/solc-static-linux /tmp/solc-0.8.20
ADD --checksum=sha256:5942c9b0934e510ee61eb3e30273f1b3fe2590df93933a93d7c58b81d19c8ff5 https://github.com/jqlang/jq/releases/download/jq-1.7.1/jq-linux-amd64 /tmp/jq
RUN echo "${SOLC_0_8_20_SHA256}  /tmp/solc-0.8.20" | sha256sum -c - \
    && echo "${JQ_SHA256}  /tmp/jq" | sha256sum -c - \
    && install -m 0755 /tmp/solc-0.8.20 /usr/local/bin/solc-0.8.20 \
    && ln -sf /usr/local/bin/solc-0.8.20 /usr/local/bin/solc \
    && install -m 0755 /tmp/jq /usr/local/bin/jq \
    && rm -f /tmp/solc-0.8.20 \
    && cd /opt/chaimir/evm-hardhat \
    && npm config set registry "${NPM_REGISTRY}" \
    && npm ci --omit=dev --ignore-scripts --no-audit --no-fund \
    && mkdir -p /workspace /runtime-state \
    && chown -R 1000:1000 /workspace /runtime-state

COPY entrypoint.sh /usr/local/bin/chaimir-hardhat-entrypoint
COPY chaimir-hardhat-adapter /usr/local/bin/chaimir-hardhat-adapter
RUN printf '%s\n' '#!/bin/sh' 'exec /usr/local/bin/node /opt/chaimir/evm-hardhat/node_modules/hardhat/dist/src/cli.js "$@"' > /usr/local/bin/hardhat-wrapper \
    && chmod +x /usr/local/bin/chaimir-hardhat-entrypoint /usr/local/bin/chaimir-hardhat-adapter /usr/local/bin/hardhat-wrapper

FROM cgr.dev/chainguard/glibc-dynamic@sha256:d0046044cd28948d3380eb0d98709dc7e63f98161fe7105135e1025650bad17a

LABEL org.opencontainers.image.title="runtime/evm-hardhat"
LABEL org.opencontainers.image.description="Chaimir Hardhat EVM teaching runtime"
ENV HOME=/workspace

# 最终层只保留运行所需的 glibc、Node、Foundry、Hardhat、Solidity 和 jq,不携带 Debian 包管理数据库。
COPY --from=build /bin/sh /bin/sh
COPY --from=build /bin/dash /bin/dash
COPY --from=build /usr/bin/env /usr/bin/env
COPY --from=build /usr/lib/x86_64-linux-gnu/libstdc++.so.6* /usr/lib/x86_64-linux-gnu/
COPY --from=build /usr/lib/x86_64-linux-gnu/libgcc_s.so.1 /usr/lib/x86_64-linux-gnu/
COPY --from=build /usr/local/bin/node /usr/local/bin/node
COPY --from=build /usr/local/bin/forge /usr/local/bin/forge
COPY --from=build /usr/local/bin/anvil /usr/local/bin/anvil
COPY --from=build /usr/local/bin/cast /usr/local/bin/cast
COPY --from=build /usr/local/bin/chisel /usr/local/bin/chisel
COPY --from=build /usr/local/bin/solc /usr/local/bin/solc
COPY --from=build /usr/local/bin/solc-0.8.20 /usr/local/bin/solc-0.8.20
COPY --from=build /usr/local/bin/jq /usr/local/bin/jq
COPY --from=build /usr/local/bin/hardhat-wrapper /usr/local/bin/hardhat
COPY --from=build /usr/local/bin/chaimir-hardhat-entrypoint /usr/local/bin/chaimir-hardhat-entrypoint
COPY --from=build /usr/local/bin/chaimir-hardhat-adapter /usr/local/bin/chaimir-hardhat-adapter
COPY --from=build --chown=1000:1000 /opt/chaimir/evm-hardhat /opt/chaimir/evm-hardhat
COPY --from=build --chown=1000:1000 /workspace /workspace
COPY --from=build --chown=1000:1000 /runtime-state /runtime-state

USER 1000:1000
WORKDIR /workspace
EXPOSE 8545
ENTRYPOINT ["/usr/local/bin/chaimir-hardhat-entrypoint"]
