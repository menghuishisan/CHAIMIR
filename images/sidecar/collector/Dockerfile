# 本文件构建混合实验数据采集 sidecar 镜像。
FROM python:3.13-alpine@sha256:399babc8b49529dabfd9c922f2b5eea81d611e4512e3ed250d75bd2e7683f4b0

LABEL org.opencontainers.image.title="sidecar/collector"
LABEL org.opencontainers.image.description="Chaimir sandbox telemetry collector sidecar"

RUN apk upgrade --no-cache \
    && addgroup -S chaimir \
    && adduser -S -G chaimir -h /collector chaimir

WORKDIR /collector
COPY collector.py /collector/collector.py

USER chaimir
ENTRYPOINT ["python", "/collector/collector.py"]
