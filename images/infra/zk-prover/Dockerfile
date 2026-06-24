# 本文件定义 ZK 证明器镜像,提供 circom/snarkjs 证明工具链。
FROM node:24-alpine3.23 AS deps

WORKDIR /opt/chaimir/zk-prover
COPY package.json package-lock.json /opt/chaimir/zk-prover/
RUN npm ci --omit=dev --ignore-scripts --no-audit --no-fund

FROM node:24-alpine3.23

LABEL org.opencontainers.image.title="infra/zk-prover"
LABEL org.opencontainers.image.description="Chaimir ZK prover toolchain image"
ENV HOME=/workspace

WORKDIR /opt/chaimir/zk-prover
COPY --from=deps /opt/chaimir/zk-prover /opt/chaimir/zk-prover
RUN apk upgrade --no-cache \
    && apk add --no-cache bash ca-certificates \
    && ln -s /opt/chaimir/zk-prover/node_modules/.bin/snarkjs /usr/local/bin/snarkjs \
    && rm -rf /usr/local/lib/node_modules/npm /usr/local/lib/node_modules/corepack /usr/local/bin/npm /usr/local/bin/npx /usr/local/bin/corepack \
    && mkdir -p /workspace \
    && chown -R 1000:1000 /workspace

COPY entrypoint.sh /usr/local/bin/chaimir-zk-prover-entrypoint
RUN chmod +x /usr/local/bin/chaimir-zk-prover-entrypoint

USER 1000:1000
WORKDIR /workspace
ENTRYPOINT ["/usr/local/bin/chaimir-zk-prover-entrypoint"]
