# 本文件定义 noVNC 图形桌面工具镜像。
FROM dorowu/ubuntu-desktop-lxde-vnc:focal

LABEL org.opencontainers.image.title="tool/novnc-desktop"
LABEL org.opencontainers.image.description="Chaimir noVNC desktop tool image"

USER root
RUN useradd --system --create-home --home-dir /workspace chaimir \
    && chown -R chaimir:chaimir /workspace

USER chaimir
WORKDIR /workspace
EXPOSE 80
