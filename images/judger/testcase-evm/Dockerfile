# 本文件定义 EVM 测试用例判题镜像。
FROM ghcr.io/foundry-rs/foundry:v1.0.0

LABEL org.opencontainers.image.title="judger/testcase-evm"
LABEL org.opencontainers.image.description="Chaimir EVM testcase judger image"

ARG HARDHAT_VERSION=2.22.19

COPY package.json package-lock.json /opt/chaimir/testcase-evm/
RUN apk add --no-cache nodejs npm git bash \
    && cd /opt/chaimir/testcase-evm \
    && npm ci --omit=dev --ignore-scripts --no-audit --no-fund \
    && ln -s /opt/chaimir/testcase-evm/node_modules/.bin/hardhat /usr/local/bin/hardhat \
    && addgroup -S chaimir \
    && adduser -S -G chaimir -h /judge chaimir \
    && chown -R chaimir:chaimir /judge

COPY run_evm_tests.sh /usr/local/bin/run-evm-tests
COPY normalize_result.py /usr/local/bin/normalize-result
RUN chmod +x /usr/local/bin/run-evm-tests /usr/local/bin/normalize-result

USER chaimir
WORKDIR /judge
ENTRYPOINT ["run-evm-tests"]
