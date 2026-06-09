# 本文件定义 EVM 测试用例判题镜像。
FROM node:20-bookworm-slim

LABEL org.opencontainers.image.title="judger/testcase-evm"
LABEL org.opencontainers.image.description="Chaimir EVM testcase judger image"

ARG HARDHAT_VERSION=2.22.19
ARG FOUNDRYUP_VERSION=nightly-7b61e0b8f4f1e5e0d0a0e93c9f64a0e3c2de7a2b

RUN groupadd --system chaimir \
    && useradd --system --gid chaimir --home-dir /judge --create-home chaimir \
    && npm install -g "hardhat@${HARDHAT_VERSION}" \
    && apt-get update \
    && apt-get install -y --no-install-recommends ca-certificates curl git \
    && curl -L https://foundry.paradigm.xyz | bash \
    && /root/.foundry/bin/foundryup --version "${FOUNDRYUP_VERSION}" \
    && cp /root/.foundry/bin/forge /usr/local/bin/forge \
    && cp /root/.foundry/bin/cast /usr/local/bin/cast \
    && rm -rf /var/lib/apt/lists/* /root/.foundry \
    && chown -R chaimir:chaimir /judge

COPY run_evm_tests.sh /usr/local/bin/run-evm-tests
COPY normalize_result.py /usr/local/bin/normalize-result
RUN chmod +x /usr/local/bin/run-evm-tests /usr/local/bin/normalize-result

USER chaimir
WORKDIR /judge
ENTRYPOINT ["run-evm-tests"]
