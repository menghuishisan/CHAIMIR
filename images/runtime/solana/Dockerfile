# 本文件定义 Solana 本地测试链运行时镜像。
FROM solanalabs/solana:v1.18.26

LABEL org.opencontainers.image.title="runtime/solana"
LABEL org.opencontainers.image.description="Chaimir Solana test validator runtime"

USER root

# 创建非 root 用户和链状态目录,测试链账本写入 runtime-state。
RUN groupadd --system chaimir \
    && useradd --system --gid chaimir --home-dir /workspace --create-home chaimir \
    && mkdir -p /runtime-state/solana \
    && chown -R chaimir:chaimir /workspace /runtime-state

COPY entrypoint.sh /usr/local/bin/chaimir-solana-entrypoint
RUN chmod +x /usr/local/bin/chaimir-solana-entrypoint

USER chaimir
WORKDIR /workspace
EXPOSE 8899
ENTRYPOINT ["/usr/local/bin/chaimir-solana-entrypoint"]
