# 本文件定义 Theia 轻量 IDE 工具镜像。
FROM theiaide/theia:1.54.0

LABEL org.opencontainers.image.title="tool/theia"
LABEL org.opencontainers.image.description="Chaimir Theia lightweight IDE image"

USER root
RUN mkdir -p /workspace \
    && chown -R theia:theia /workspace

USER theia
WORKDIR /workspace
EXPOSE 3000
CMD ["yarn", "theia", "start", "/workspace", "--hostname=0.0.0.0"]
