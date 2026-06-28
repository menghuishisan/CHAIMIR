# 本文件定义 Solana 本地测试链运行时镜像。
FROM solanalabs/solana:v1.18.26 AS solana-tools

FROM debian:13.5-slim AS runtime-libs
RUN apt-get -o Acquire::Retries=3 update \
    && apt-get -o Acquire::Retries=3 -y install --no-install-recommends ca-certificates libstdc++6 libudev1 \
    && rm -rf /var/lib/apt/lists/*

FROM alpine:3.23 AS runtime-dirs
RUN mkdir -p /workspace /runtime-state/solana/ledger \
    && chown -R 1000:1000 /workspace /runtime-state

FROM scratch

LABEL org.opencontainers.image.title="runtime/solana"
LABEL org.opencontainers.image.description="Chaimir Solana test validator runtime"

COPY --from=runtime-libs /lib/x86_64-linux-gnu/ /lib/x86_64-linux-gnu/
COPY --from=runtime-libs /lib64/ /lib64/
COPY --from=runtime-libs /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/ca-certificates.crt
COPY --from=runtime-dirs --chown=1000:1000 /workspace /workspace
COPY --from=runtime-dirs --chown=1000:1000 /runtime-state /runtime-state
COPY --from=solana-tools /usr/bin/solana* /usr/local/bin/

USER 1000:1000
WORKDIR /workspace
EXPOSE 8899
ENTRYPOINT ["/usr/local/bin/solana-test-validator"]
CMD ["--ledger", "/runtime-state/solana/ledger", "--rpc-port", "8899", "--bind-address", "0.0.0.0"]
