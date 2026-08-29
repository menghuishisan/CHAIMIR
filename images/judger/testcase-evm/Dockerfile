# 本文件定义 EVM 测试用例判题镜像。
ARG JUDGE_MIN_IMAGE=invalid.invalid/base/judge-min:required

FROM ghcr.io/foundry-rs/foundry:v1.7.1@sha256:bdc584033cefab8be282c7f57c263cda42f04700cd1ec88d64bebe2f645aba72 AS foundry
FROM ${JUDGE_MIN_IMAGE} AS judge-base

FROM node:24-trixie-slim@sha256:db4f733c26868756bae92ac764d7e99bf31809ce7db886ed525960fe7d544601 AS node-build

ARG HTTP_PROXY
ARG HTTPS_PROXY
ARG NO_PROXY

LABEL org.opencontainers.image.title="judger/testcase-evm"
LABEL org.opencontainers.image.description="Chaimir EVM testcase judger image"
ENV HOME=/judge
ENV XDG_CACHE_HOME=/opt/chaimir/cache
ENV HARDHAT_TELEMETRY_DISABLED=true

ARG HARDHAT_VERSION=3.9.0
ARG SOLC_0_8_20_SHA256=0479d44fdf9c501c25337fdc540419f1593b884a87b47f023da4f1c700fda782
ARG NPM_CONFIG_REGISTRY=https://registry.npmmirror.com

USER root

COPY --from=foundry /usr/local/bin/forge /usr/local/bin/forge
COPY --from=foundry /usr/local/bin/anvil /usr/local/bin/anvil
COPY --from=foundry /usr/local/bin/cast /usr/local/bin/cast
COPY --from=foundry /usr/local/bin/chisel /usr/local/bin/chisel
COPY --from=judge-base /usr/local/bin/normalize-result /usr/local/bin/normalize-result
COPY package.json package-lock.json /opt/chaimir/testcase-evm/
RUN export http_proxy="${HTTP_PROXY}" https_proxy="${HTTPS_PROXY}" no_proxy="${NO_PROXY}" \
    && for attempt in 1 2 3 4 5 6 7 8; do \
        if apt-get -o Acquire::Retries=3 -o Acquire::http::Pipeline-Depth=0 update; then break; fi; \
        if [ "${attempt}" -eq 8 ]; then exit 1; fi; \
        sleep $((attempt * 5)); \
    done \
    && for attempt in 1 2 3 4 5 6 7 8; do \
        if apt-get -o Acquire::Retries=3 -o Acquire::http::Pipeline-Depth=0 -y dist-upgrade; then break; fi; \
        if [ "${attempt}" -eq 8 ]; then exit 1; fi; \
        sleep $((attempt * 5)); \
    done \
    && for attempt in 1 2 3 4 5 6 7 8; do \
        if apt-get -o Acquire::Retries=3 -o Acquire::http::Pipeline-Depth=0 install -y --no-install-recommends bash ca-certificates wget; then break; fi; \
        if [ "${attempt}" -eq 8 ]; then exit 1; fi; \
        sleep $((attempt * 5)); \
    done \
    && for attempt in 1 2 3 4 5 6 7 8; do \
        if wget -q --timeout=300 --tries=3 -O /tmp/solc-0.8.20 https://github.com/ethereum/solidity/releases/download/v0.8.20/solc-static-linux \
            && echo "${SOLC_0_8_20_SHA256}  /tmp/solc-0.8.20" | sha256sum -c -; then break; fi; \
        if [ "${attempt}" -eq 8 ]; then exit 1; fi; \
        rm -f /tmp/solc-0.8.20; \
        sleep $((attempt * 5)); \
    done \
    && install -m 0755 /tmp/solc-0.8.20 /usr/local/bin/solc-0.8.20 \
    && ln -sf /usr/local/bin/solc-0.8.20 /usr/local/bin/solc \
    && rm -f /tmp/solc-0.8.20 \
    && cd /opt/chaimir/testcase-evm \
    && npm config set registry "${NPM_CONFIG_REGISTRY}" \
    && for attempt in 1 2 3 4 5 6 7 8; do \
        if npm ci --omit=dev --ignore-scripts --no-audit --no-fund --fetch-timeout=120000 --fetch-retries=8; then break; fi; \
        if [ "${attempt}" -eq 8 ]; then exit 1; fi; \
        sleep $((attempt * 5)); \
    done \
    && ln -s /opt/chaimir/testcase-evm/node_modules/.bin/hardhat /usr/local/bin/hardhat \
    && mkdir -p /tmp/chaimir-hardhat-cache/contracts /tmp/chaimir-hardhat-cache/node_modules/.bin /opt/chaimir/cache /judge \
    && printf '%s\n' '{"type":"module"}' > /tmp/chaimir-hardhat-cache/package.json \
    && printf '%s\n' 'export default { solidity: "0.8.20" };' > /tmp/chaimir-hardhat-cache/hardhat.config.js \
    && printf '%s\n' '// SPDX-License-Identifier: MIT' 'pragma solidity ^0.8.20;' 'contract CacheWarmup {}' > /tmp/chaimir-hardhat-cache/contracts/CacheWarmup.sol \
    && ln -s /opt/chaimir/testcase-evm/node_modules/hardhat /tmp/chaimir-hardhat-cache/node_modules/hardhat \
    && ln -s /opt/chaimir/testcase-evm/node_modules/.bin/hardhat /tmp/chaimir-hardhat-cache/node_modules/.bin/hardhat \
    && cd /tmp/chaimir-hardhat-cache \
    && ./node_modules/.bin/hardhat compile \
    && rm -rf /tmp/chaimir-hardhat-cache \
    && rm -rf /usr/local/lib/node_modules/npm /usr/local/lib/node_modules/corepack /usr/local/bin/npm /usr/local/bin/npx /usr/local/bin/corepack \
    && chown -R 1000:1000 /judge /opt/chaimir/cache \
    && rm -rf /var/lib/apt/lists/*

COPY run_evm_tests.sh /usr/local/bin/run-evm-tests
RUN chmod +x /usr/local/bin/run-evm-tests /usr/local/bin/normalize-result

FROM alpine:3.23@sha256:fd791d74b68913cbb027c6546007b3f0d3bc45125f797758156952bc2d6daf40

RUN apk add --no-cache \
    'libcrypto3>=3.5.8-r0' \
    'libssl3>=3.5.8-r0'

LABEL org.opencontainers.image.title="judger/testcase-evm"
LABEL org.opencontainers.image.description="Chaimir EVM testcase judger image"
ENV HOME=/judge
ENV XDG_CACHE_HOME=/opt/chaimir/cache
ENV HARDHAT_TELEMETRY_DISABLED=true
ENV PATH=/usr/local/bin:/usr/bin:/bin

COPY --from=foundry /usr/local/bin/forge /usr/local/bin/forge
COPY --from=foundry /usr/local/bin/anvil /usr/local/bin/anvil
COPY --from=foundry /usr/local/bin/cast /usr/local/bin/cast
COPY --from=foundry /usr/local/bin/chisel /usr/local/bin/chisel
COPY --from=judge-base /usr/local/bin/normalize-result /usr/local/bin/normalize-result
COPY --from=node-build /usr/local/bin/node /usr/local/bin/node
COPY --from=node-build /opt/chaimir/testcase-evm /opt/chaimir/testcase-evm
COPY --from=node-build /usr/local/bin/solc-0.8.20 /usr/local/bin/solc-0.8.20
COPY --from=node-build /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/ca-certificates.crt
COPY --from=node-build /lib/x86_64-linux-gnu/libdl.so.2 /lib/x86_64-linux-gnu/libdl.so.2
COPY --from=node-build /lib/x86_64-linux-gnu/libstdc++.so.6 /lib/x86_64-linux-gnu/libstdc++.so.6
COPY --from=node-build /lib/x86_64-linux-gnu/libm.so.6 /lib/x86_64-linux-gnu/libm.so.6
COPY --from=node-build /lib/x86_64-linux-gnu/libgcc_s.so.1 /lib/x86_64-linux-gnu/libgcc_s.so.1
COPY --from=node-build /lib/x86_64-linux-gnu/libpthread.so.0 /lib/x86_64-linux-gnu/libpthread.so.0
COPY --from=node-build /lib/x86_64-linux-gnu/libc.so.6 /lib/x86_64-linux-gnu/libc.so.6
COPY --from=node-build /lib64/ld-linux-x86-64.so.2 /lib64/ld-linux-x86-64.so.2
COPY --from=node-build /usr/local/bin/run-evm-tests /usr/local/bin/run-evm-tests

RUN ln -sf /usr/local/bin/solc-0.8.20 /usr/local/bin/solc \
    && ln -sf /opt/chaimir/testcase-evm/node_modules/.bin/hardhat /usr/local/bin/hardhat \
    && mkdir -p /opt/chaimir/cache /judge /tmp \
    && chown -R 1000:1000 /opt/chaimir /judge /tmp

USER 1000:1000
WORKDIR /judge
ENTRYPOINT ["run-evm-tests"]
