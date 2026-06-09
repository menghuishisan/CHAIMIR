# 本文件定义 Mythril 符号执行安全分析工具镜像。
FROM mythril/myth:0.24.8

LABEL org.opencontainers.image.title="tool/mythril"
LABEL org.opencontainers.image.description="Chaimir Mythril symbolic execution tool image"

USER root
RUN addgroup -S chaimir \
    && adduser -S -G chaimir -h /workspace chaimir \
    && chown -R chaimir:chaimir /workspace

USER chaimir
WORKDIR /workspace
ENTRYPOINT ["myth"]
