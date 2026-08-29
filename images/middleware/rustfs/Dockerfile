# 本文件在固定 RustFS 官方镜像上更新 Alpine OpenSSL 安全包，保持 S3 服务行为不变。
FROM rustfs/rustfs:1.0.0-rc.3@sha256:800cf3f352a0a27e3275ca854a51f0027975d7acc7a0d52089a35bcc9fcbf0b5

USER root
RUN apk add --no-cache \
        openssl=3.5.8-r0 \
        libssl3=3.5.8-r0 \
        libcrypto3=3.5.8-r0

USER 10001:10001
