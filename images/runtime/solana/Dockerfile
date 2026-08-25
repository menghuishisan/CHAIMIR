# 本文件定义 Solana 本地测试链运行时镜像,复用官方已验证运行层并统一 Chaimir 入口。
FROM solanalabs/solana:v1.18.26@sha256:098806e64d44bccdbedbf07c2edabd1c850b92c8a4a0f81eba9c789034813db6 AS solana-upstream

FROM debian:13-slim@sha256:abc9cb88a5587630d7f915f47b23b0668fe250fbfc6457aa4d52b534c1bbf73f

# Keep the pinned Solana toolchain but rebuild the runtime on current Debian
# so the upstream Debian 11 base libraries are not carried into the platform.
RUN sed -i 's|http://deb.debian.org/debian$|http://ftp.de.debian.org/debian|' /etc/apt/sources.list.d/debian.sources \
    && apt-get -o Acquire::Retries=8 update \
    && DEBIAN_FRONTEND=noninteractive apt-get install -y --no-install-recommends \
        bash ca-certificates libstdc++6 libudev1 \
    && rm -rf /var/lib/apt/lists/* \
    && mkdir -p /workspace /tmp /runtime-state/solana/ledger \
    && chown -R 1000:1000 /workspace /tmp /runtime-state

COPY --from=solana-upstream /usr/bin/solana* /usr/bin/
COPY --from=solana-upstream /usr/bin/spl-* /usr/bin/

LABEL org.opencontainers.image.title="runtime/solana"
LABEL org.opencontainers.image.description="Chaimir Solana test validator runtime"

COPY entrypoint.sh /usr/local/bin/chaimir-solana-entrypoint
RUN chmod 0755 /usr/local/bin/chaimir-solana-entrypoint

USER 1000:1000
WORKDIR /workspace
EXPOSE 8899
ENTRYPOINT ["/usr/local/bin/chaimir-solana-entrypoint"]
