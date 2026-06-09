# 本文件构建 gas 消耗分析工具镜像。
FROM python:3.13-alpine

LABEL org.opencontainers.image.title="tool/gas-profiler"
LABEL org.opencontainers.image.description="Chaimir gas profiler command line tool"

RUN addgroup -S chaimir \
    && adduser -S -G chaimir -h /workspace chaimir

WORKDIR /workspace
COPY gas_profiler.py /usr/local/bin/gas-profiler
RUN chmod +x /usr/local/bin/gas-profiler

USER chaimir
ENTRYPOINT ["gas-profiler"]
