# 本文件构建后端图布局仿真镜像。
FROM python:3.13-alpine@sha256:399babc8b49529dabfd9c922f2b5eea81d611e4512e3ed250d75bd2e7683f4b0

LABEL org.opencontainers.image.title="sim/graph-layout"
LABEL org.opencontainers.image.description="Chaimir backend graph layout simulation image"

RUN apk upgrade --no-cache \
    && addgroup -S chaimir \
    && adduser -S -G chaimir -h /sim chaimir

WORKDIR /sim
COPY graph_layout.py /sim/graph_layout.py

USER chaimir
ENTRYPOINT ["python", "/sim/graph_layout.py"]
