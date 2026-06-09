# 本文件定义 ZK 证明器镜像,提供 circom/snarkjs 证明工具链。
FROM node:20-bookworm-slim

LABEL org.opencontainers.image.title="infra/zk-prover"
LABEL org.opencontainers.image.description="Chaimir ZK prover toolchain image"

WORKDIR /opt/chaimir/zk-prover
COPY package.json package-lock.json /opt/chaimir/zk-prover/
RUN useradd --system --create-home --home-dir /workspace chaimir \
    && npm ci --omit=dev --ignore-scripts --no-audit --no-fund \
    && ln -s /opt/chaimir/zk-prover/node_modules/.bin/snarkjs /usr/local/bin/snarkjs \
    && chown -R chaimir:chaimir /workspace

COPY entrypoint.sh /usr/local/bin/chaimir-zk-prover-entrypoint
RUN chmod +x /usr/local/bin/chaimir-zk-prover-entrypoint

USER chaimir
WORKDIR /workspace
ENTRYPOINT ["/usr/local/bin/chaimir-zk-prover-entrypoint"]
