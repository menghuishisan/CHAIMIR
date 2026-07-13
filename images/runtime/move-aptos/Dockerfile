# 本文件定义 Aptos Move 本地网运行时镜像,从官方工具镜像提取 Aptos CLI 到干净运行层。
FROM aptoslabs/tools:aptos-node-v1.46.2@sha256:af715f809bd12769ba53750f81a1eb8384b312babd472ef6216001652600a326 AS aptos-tools

FROM debian:13.5-slim@sha256:28de0877c2189802884ccd20f15ee41c203573bd87bb6b883f5f46362d24c5c2 AS runtime-libs
RUN apt-get -o Acquire::Retries=3 update \
    && apt-get -o Acquire::Retries=3 -y install --no-install-recommends ca-certificates libstdc++6 libudev1 libssl3t64 libcap2 zlib1g libzstd1 \
    && rm -rf /var/lib/apt/lists/*

FROM alpine:3.23@sha256:fd791d74b68913cbb027c6546007b3f0d3bc45125f797758156952bc2d6daf40 AS runtime-dirs
RUN mkdir -p /workspace /runtime-state/aptos \
    && chown -R 1000:1000 /workspace /runtime-state

FROM scratch

LABEL org.opencontainers.image.title="runtime/move-aptos"
LABEL org.opencontainers.image.description="Chaimir Aptos Move localnet runtime"

COPY --from=runtime-libs /lib/x86_64-linux-gnu/ /lib/x86_64-linux-gnu/
COPY --from=runtime-libs /lib64/ /lib64/
COPY --from=runtime-libs /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/ca-certificates.crt
COPY --from=runtime-dirs --chown=1000:1000 /workspace /workspace
COPY --from=runtime-dirs --chown=1000:1000 /runtime-state /runtime-state
COPY --from=aptos-tools /usr/local/bin/aptos /usr/local/bin/aptos
COPY --from=aptos-tools /aptos /aptos

USER 1000:1000
WORKDIR /workspace
EXPOSE 8080
ENTRYPOINT ["/usr/local/bin/aptos"]
CMD ["node", "run-local-testnet", "--test-dir", "/runtime-state/aptos", "--with-faucet", "--force-restart", "--assume-yes", "--bind-to", "0.0.0.0"]
