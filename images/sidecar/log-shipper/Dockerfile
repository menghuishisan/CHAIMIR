# 本文件构建沙箱日志采集 sidecar 镜像。
FROM python:3.13-alpine@sha256:399babc8b49529dabfd9c922f2b5eea81d611e4512e3ed250d75bd2e7683f4b0

LABEL org.opencontainers.image.title="sidecar/log-shipper"
LABEL org.opencontainers.image.description="Chaimir sandbox log shipper sidecar image"

# 基础镜像的系统库必须显式锁定到 Trivy 修复版本，避免把已知漏洞带入运行时层。
RUN apk add --no-cache --timeout 300 \
    libuuid=2.42.3-r1 \
    libcrypto3=3.5.8-r0 \
    libssl3=3.5.8-r0

RUN addgroup -S chaimir \
    && adduser -S -G chaimir -h /app chaimir

WORKDIR /app
COPY log_shipper.py /app/log_shipper.py

USER chaimir
CMD ["python", "/app/log_shipper.py"]
