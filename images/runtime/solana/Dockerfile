# 本文件定义 Solana 本地测试链运行时镜像,复用官方已验证运行层并统一 Chaimir 入口。
FROM solanalabs/solana:v1.18.26@sha256:098806e64d44bccdbedbf07c2edabd1c850b92c8a4a0f81eba9c789034813db6

LABEL org.opencontainers.image.title="runtime/solana"
LABEL org.opencontainers.image.description="Chaimir Solana test validator runtime"

RUN groupadd --gid 1000 chaimir \
    && useradd --uid 1000 --gid 1000 --home-dir /workspace --create-home --shell /usr/sbin/nologin chaimir \
    && mkdir -p /workspace /tmp /runtime-state/solana/ledger \
    && chown -R 1000:1000 /workspace /tmp /runtime-state

COPY entrypoint.sh /usr/local/bin/chaimir-solana-entrypoint
RUN chmod 0755 /usr/local/bin/chaimir-solana-entrypoint

USER 1000:1000
WORKDIR /workspace
EXPOSE 8899
ENTRYPOINT ["/usr/local/bin/chaimir-solana-entrypoint"]
