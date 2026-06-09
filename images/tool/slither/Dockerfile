# 本文件定义 Slither 工具镜像,复用成熟安全工具箱。
FROM trailofbits/eth-security-toolbox:2024.12.0

LABEL org.opencontainers.image.title="tool/slither"
LABEL org.opencontainers.image.description="Chaimir Slither security scanner image"

USER root
RUN useradd --system --create-home --home-dir /workspace chaimir \
    && chown -R chaimir:chaimir /workspace

USER chaimir
WORKDIR /workspace
CMD ["slither", "--version"]
