# 本文件构建后端图布局仿真镜像。
FROM python:3.13-alpine@sha256:399babc8b49529dabfd9c922f2b5eea81d611e4512e3ed250d75bd2e7683f4b0

LABEL org.opencontainers.image.title="sim/graph-layout"
LABEL org.opencontainers.image.description="Chaimir backend graph layout simulation image"

# 基础镜像的系统库必须显式锁定到 Trivy 修复版本，避免已知漏洞进入仿真运行时。
RUN apk add --no-cache --timeout 300 \
    libuuid=2.42.3-r1 \
    libcrypto3=3.5.8-r0 \
    libssl3=3.5.8-r0

RUN addgroup -S chaimir \
    && adduser -S -G chaimir -h /sim chaimir

WORKDIR /sim
COPY graph_layout.py /sim/graph_layout.py

USER chaimir
ENTRYPOINT ["python", "/sim/graph_layout.py"]
