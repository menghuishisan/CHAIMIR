# 本文件定义 noVNC 图形桌面工具镜像。
FROM dorowu/ubuntu-desktop-lxde-vnc:focal

LABEL org.opencontainers.image.title="tool/novnc-desktop"
LABEL org.opencontainers.image.description="Chaimir noVNC desktop tool image"

USER root
RUN groupadd --gid 1000 chaimir \
    && useradd --uid 1000 --gid 1000 --home-dir /workspace --create-home --shell /usr/sbin/nologin chaimir \
    && chown -R 1000:1000 /workspace

USER 1000:1000
WORKDIR /workspace
EXPOSE 80
