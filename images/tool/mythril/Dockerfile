# 本文件定义 Mythril 符号执行安全分析工具镜像。
FROM mythril/myth:0.24.8

LABEL org.opencontainers.image.title="tool/mythril"
LABEL org.opencontainers.image.description="Chaimir Mythril symbolic execution tool image"

USER root
RUN if ! getent group 1000 >/dev/null; then groupadd --gid 1000 chaimir; fi \
    && if ! id -u 1000 >/dev/null 2>&1; then useradd --uid 1000 --gid 1000 --home-dir /workspace --create-home --shell /usr/sbin/nologin chaimir; fi \
    && mkdir -p /workspace \
    && chown -R 1000:1000 /workspace

USER 1000:1000
WORKDIR /workspace
ENTRYPOINT ["myth"]
