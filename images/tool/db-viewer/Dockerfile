# 本文件定义数据库可视化工具镜像。
FROM sosedoff/pgweb:0.16.2

LABEL org.opencontainers.image.title="tool/db-viewer"
LABEL org.opencontainers.image.description="Chaimir database viewer tool image"

USER root
RUN apk upgrade --no-cache \
    && if ! getent group 1000 >/dev/null; then groupadd --gid 1000 chaimir; fi \
    && if ! id -u 1000 >/dev/null 2>&1; then useradd --uid 1000 --gid 1000 --home-dir /workspace --create-home --shell /usr/sbin/nologin chaimir; fi \
    && mkdir -p /workspace \
    && chown -R 1000:1000 /workspace

USER 1000:1000
WORKDIR /workspace
EXPOSE 8081
CMD ["pgweb", "--bind=0.0.0.0", "--listen=8081"]
