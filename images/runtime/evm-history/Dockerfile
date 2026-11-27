ARG EVM_GETH_IMAGE=invalid.invalid/runtime/evm-geth:required
FROM ${EVM_GETH_IMAGE}

LABEL org.opencontainers.image.title="runtime/evm-history"
LABEL org.opencontainers.image.description="Chaimir immutable complete Ethereum history state provider"

# 构建材料必须包含 snapshot.json 与已关闭的 hash-scheme chaindata；缺失不得生成普通链镜像。
COPY --chown=1000:1000 snapshot/snapshot.json /usr/local/share/chaimir/history/snapshot.json
COPY --chown=1000:1000 snapshot/chaindata/ /usr/local/share/chaimir/history/chaindata/
RUN chaimir-evm-history --verify

ENTRYPOINT ["/usr/local/bin/chaimir-evm-history"]
