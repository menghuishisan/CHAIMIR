# 本文件定义 Mythril 符号执行安全分析工具镜像。
FROM mythril/myth:0.24.8

LABEL org.opencontainers.image.title="tool/mythril"
LABEL org.opencontainers.image.description="Chaimir Mythril symbolic execution tool image"

USER root
RUN addgroup -S -g 1000 chaimir \
    && adduser -S -u 1000 -G chaimir -h /workspace chaimir \
    && chown -R 1000:1000 /workspace

USER 1000:1000
WORKDIR /workspace
ENTRYPOINT ["myth"]
