# 本文件构建 gas 消耗分析工具镜像。
FROM python:3.13-alpine@sha256:399babc8b49529dabfd9c922f2b5eea81d611e4512e3ed250d75bd2e7683f4b0

LABEL org.opencontainers.image.title="tool/gas-profiler"
LABEL org.opencontainers.image.description="Chaimir gas profiler command line tool"

RUN apk upgrade --no-cache \
    && addgroup -S -g 1000 chaimir \
    && adduser -S -u 1000 -G chaimir -h /workspace chaimir

WORKDIR /workspace
COPY gas_profiler.py /usr/local/bin/gas-profiler
RUN chmod +x /usr/local/bin/gas-profiler

USER 1000:1000
ENTRYPOINT ["gas-profiler"]
