# 本文件定义 ZK 证明器镜像,提供 circom/snarkjs 证明工具链。
FROM node:20-bookworm-slim

LABEL org.opencontainers.image.title="infra/zk-prover"
LABEL org.opencontainers.image.description="Chaimir ZK prover toolchain image"

RUN useradd --system --create-home --home-dir /workspace chaimir \
    && npm install -g snarkjs@0.7.5 circom_tester@0.0.24 \
    && chown -R chaimir:chaimir /workspace

USER chaimir
WORKDIR /workspace
CMD ["snarkjs", "--help"]
