# 本文件定义 Slither 工具镜像,复用成熟安全工具箱。
FROM trailofbits/eth-security-toolbox:nightly-20260608

LABEL org.opencontainers.image.title="tool/slither"
LABEL org.opencontainers.image.description="Chaimir Slither security scanner image"

USER root
RUN if ! getent group 1000 >/dev/null; then groupadd --gid 1000 chaimir; fi \
    && if ! id -u 1000 >/dev/null 2>&1; then useradd --uid 1000 --gid 1000 --home-dir /workspace --create-home --shell /usr/sbin/nologin chaimir; fi \
    && mkdir -p /workspace \
    && chown -R 1000:1000 /workspace

USER 1000:1000
WORKDIR /workspace
CMD ["slither", "--version"]
