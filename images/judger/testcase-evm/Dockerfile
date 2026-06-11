# 本文件定义 EVM 测试用例判题镜像。
FROM ghcr.io/foundry-rs/foundry:v1.0.0 AS foundry

FROM node:22-bookworm-slim

LABEL org.opencontainers.image.title="judger/testcase-evm"
LABEL org.opencontainers.image.description="Chaimir EVM testcase judger image"
ENV HOME=/judge
ENV XDG_CACHE_HOME=/opt/chaimir/cache
ENV HARDHAT_TELEMETRY_DISABLED=true

ARG HARDHAT_VERSION=3.9.0

USER root

COPY --from=foundry /usr/local/bin/forge /usr/local/bin/forge
COPY --from=foundry /usr/local/bin/anvil /usr/local/bin/anvil
COPY --from=foundry /usr/local/bin/cast /usr/local/bin/cast
COPY --from=foundry /usr/local/bin/chisel /usr/local/bin/chisel
COPY package.json package-lock.json /opt/chaimir/testcase-evm/
RUN apt-get update \
    && apt-get install -y --no-install-recommends git bash ca-certificates python3 \
    && rm -rf /var/lib/apt/lists/* \
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
    && chown -R 1000:1000 /judge /opt/chaimir/cache

COPY run_evm_tests.sh /usr/local/bin/run-evm-tests
COPY normalize_result.py /usr/local/bin/normalize-result
RUN chmod +x /usr/local/bin/run-evm-tests /usr/local/bin/normalize-result

USER 1000:1000
WORKDIR /judge
ENTRYPOINT ["run-evm-tests"]
