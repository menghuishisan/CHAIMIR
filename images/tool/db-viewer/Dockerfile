# 本文件定义数据库可视化工具镜像。
FROM sosedoff/pgweb:0.16.2

LABEL org.opencontainers.image.title="tool/db-viewer"
LABEL org.opencontainers.image.description="Chaimir database viewer tool image"

USER root
RUN addgroup -S -g 1000 chaimir \
    && adduser -S -u 1000 -G chaimir -h /workspace chaimir

USER 1000:1000
WORKDIR /workspace
EXPOSE 8081
CMD ["pgweb", "--bind=0.0.0.0", "--listen=8081"]
