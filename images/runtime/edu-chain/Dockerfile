# 本文件定义 Chaimir 自研教学链运行时镜像。
FROM python:3.13-alpine

LABEL org.opencontainers.image.title="runtime/edu-chain"
LABEL org.opencontainers.image.description="Chaimir deterministic teaching chain runtime"

RUN addgroup -S -g 1000 chaimir \
    && adduser -S -u 1000 -G chaimir -h /workspace chaimir \
    && mkdir -p /runtime-state/edu-chain \
    && chown -R 1000:1000 /workspace /runtime-state

COPY edu_chain.py /app/edu_chain.py

USER 1000:1000
WORKDIR /workspace
EXPOSE 8080
CMD ["python", "/app/edu_chain.py"]
