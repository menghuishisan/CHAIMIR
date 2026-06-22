# 本文件构建沙箱日志采集 sidecar 镜像。
FROM python:3.13-alpine

LABEL org.opencontainers.image.title="sidecar/log-shipper"
LABEL org.opencontainers.image.description="Chaimir sandbox log shipper sidecar image"

RUN apk upgrade --no-cache \
    && addgroup -S chaimir \
    && adduser -S -G chaimir -h /app chaimir

WORKDIR /app
COPY log_shipper.py /app/log_shipper.py

USER chaimir
CMD ["python", "/app/log_shipper.py"]
