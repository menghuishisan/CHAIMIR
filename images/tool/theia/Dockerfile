# 本文件定义 Theia 轻量 IDE 工具镜像。
FROM ghcr.io/eclipse-theia/theia-blueprint/theia-ide:1.54.0

LABEL org.opencontainers.image.title="tool/theia"
LABEL org.opencontainers.image.description="Chaimir Theia lightweight IDE image"

USER root
RUN mkdir -p /workspace /home/theia/.theia-ide /home/theia/.theia /home/theia/.cache \
    && chown -R 1000:1000 /workspace /home/theia

ENV HOME=/home/theia
USER 1000:1000
WORKDIR /workspace
EXPOSE 3000
CMD ["yarn", "theia", "start", "/workspace", "--hostname=0.0.0.0"]
