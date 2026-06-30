# 本文件定义 Sui Move 本地网运行时镜像。
FROM mysten/sui-tools:086c222823a48f5136eedf969e9d489dc265e714 AS sui-tools

FROM debian:13.5-slim@sha256:28de0877c2189802884ccd20f15ee41c203573bd87bb6b883f5f46362d24c5c2 AS runtime-libs
RUN apt-get -o Acquire::Retries=3 update \
    && apt-get -o Acquire::Retries=3 -y install --no-install-recommends ca-certificates libstdc++6 \
    && rm -rf /var/lib/apt/lists/*

FROM alpine:3.23 AS runtime-dirs
RUN mkdir -p /workspace /runtime-state/sui \
    && chown -R 1000:1000 /workspace /runtime-state

FROM scratch

LABEL org.opencontainers.image.title="runtime/move-sui"
LABEL org.opencontainers.image.description="Chaimir Sui Move localnet runtime"

COPY --from=runtime-libs /lib/x86_64-linux-gnu/ /lib/x86_64-linux-gnu/
COPY --from=runtime-libs /lib64/ /lib64/
COPY --from=runtime-libs /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/ca-certificates.crt
COPY --from=runtime-dirs --chown=1000:1000 /workspace /workspace
COPY --from=runtime-dirs --chown=1000:1000 /runtime-state /runtime-state
COPY --from=sui-tools /usr/local/bin/sui /usr/local/bin/sui
COPY --from=sui-tools /usr/local/bin/sui-faucet /usr/local/bin/sui-faucet
COPY --from=sui-tools /usr/local/bin/sui-tool /usr/local/bin/sui-tool

USER 1000:1000
WORKDIR /workspace
EXPOSE 9000
ENTRYPOINT ["/usr/local/bin/sui"]
CMD ["start", "--with-faucet", "--force-regenesis", "--network.config", "/runtime-state/sui/network.yaml"]
