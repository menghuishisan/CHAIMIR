# 本文件定义 EVM 测试用例判题镜像。
ARG NODE_BUILDER_IMAGE=invalid.invalid/base/node-builder:required
ARG JUDGE_MIN_IMAGE=invalid.invalid/base/judge-min:required

FROM ghcr.io/foundry-rs/foundry:v1.7.1@sha256:bdc584033cefab8be282c7f57c263cda42f04700cd1ec88d64bebe2f645aba72 AS foundry
FROM ${JUDGE_MIN_IMAGE} AS judge-base

FROM ${NODE_BUILDER_IMAGE}

LABEL org.opencontainers.image.title="judger/testcase-evm"
LABEL org.opencontainers.image.description="Chaimir EVM testcase judger image"
ENV HOME=/judge
ENV XDG_CACHE_HOME=/opt/chaimir/cache
ENV HARDHAT_TELEMETRY_DISABLED=true

ARG HARDHAT_VERSION=3.9.0
ARG SOLC_0_8_20_SHA256=0479d44fdf9c501c25337fdc540419f1593b884a87b47f023da4f1c700fda782

USER root
ADD https://github.com/ethereum/solidity/releases/download/v0.8.20/solc-static-linux /tmp/solc-0.8.20

COPY --from=foundry /usr/local/bin/forge /usr/local/bin/forge
COPY --from=foundry /usr/local/bin/anvil /usr/local/bin/anvil
COPY --from=foundry /usr/local/bin/cast /usr/local/bin/cast
COPY --from=foundry /usr/local/bin/chisel /usr/local/bin/chisel
COPY --from=judge-base /usr/local/bin/normalize-result /usr/local/bin/normalize-result
COPY package.json package-lock.json /opt/chaimir/testcase-evm/
RUN apk upgrade --no-cache \
    && apk add --no-cache bash ca-certificates \
    && echo "${SOLC_0_8_20_SHA256}  /tmp/solc-0.8.20" | sha256sum -c - \
    && install -m 0755 /tmp/solc-0.8.20 /usr/local/bin/solc-0.8.20 \
    && ln -sf /usr/local/bin/solc-0.8.20 /usr/local/bin/solc \
    && rm -f /tmp/solc-0.8.20 \
    && cd /opt/chaimir/testcase-evm \
    && npm ci --omit=dev --ignore-scripts --no-audit --no-fund \
    && ln -s /opt/chaimir/testcase-evm/node_modules/.bin/hardhat /usr/local/bin/hardhat \
    && mkdir -p /tmp/chaimir-hardhat-cache/contracts /tmp/chaimir-hardhat-cache/node_modules/.bin /opt/chaimir/cache /judge \
    && printf '%s\n' '{"type":"module"}' > /tmp/chaimir-hardhat-cache/package.json \
    && printf '%s\n' 'export default { solidity: "0.8.26" };' > /tmp/chaimir-hardhat-cache/hardhat.config.js \
    && printf '%s\n' '// SPDX-License-Identifier: MIT' 'pragma solidity ^0.8.26;' 'contract CacheWarmup {}' > /tmp/chaimir-hardhat-cache/contracts/CacheWarmup.sol \
    && ln -s /opt/chaimir/testcase-evm/node_modules/hardhat /tmp/chaimir-hardhat-cache/node_modules/hardhat \
    && ln -s /opt/chaimir/testcase-evm/node_modules/.bin/hardhat /tmp/chaimir-hardhat-cache/node_modules/.bin/hardhat \
    && cd /tmp/chaimir-hardhat-cache \
    && ./node_modules/.bin/hardhat compile \
    && rm -rf /tmp/chaimir-hardhat-cache \
    && rm -rf /usr/local/lib/node_modules/npm /usr/local/lib/node_modules/corepack /usr/local/bin/npm /usr/local/bin/npx /usr/local/bin/corepack \
    && chown -R 1000:1000 /judge /opt/chaimir/cache

COPY run_evm_tests.sh /usr/local/bin/run-evm-tests
RUN chmod +x /usr/local/bin/run-evm-tests /usr/local/bin/normalize-result

USER 1000:1000
WORKDIR /judge
ENTRYPOINT ["run-evm-tests"]
