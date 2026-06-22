# 本文件构建沙箱初始化容器镜像。
FROM alpine:3.22

LABEL org.opencontainers.image.title="init/sandbox-seed"
LABEL org.opencontainers.image.description="Chaimir sandbox workspace seeding init image"

RUN apk upgrade --no-cache \
    && addgroup -S chaimir \
    && adduser -S -G chaimir -h /seed chaimir

WORKDIR /seed
COPY seed.sh /usr/local/bin/chaimir-sandbox-seed
RUN chmod +x /usr/local/bin/chaimir-sandbox-seed

USER chaimir
ENTRYPOINT ["/usr/local/bin/chaimir-sandbox-seed"]
