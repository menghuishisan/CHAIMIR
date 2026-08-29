# 本文件定义 ClamAV clamd 的安全运行镜像,基于官方 Alpine 运行层保留 INSTREAM 扫描能力。
FROM docker.io/clamav/clamav@sha256:594d54aff5ad3b1a5211d67cdc6d49a9a332276da35d821c3dd4cf6c4cdf6237

RUN apk add --no-cache \
    'libcrypto3>=3.5.8-r0' \
    'libssl3>=3.5.8-r0'

LABEL org.opencontainers.image.title="middleware/clamav"
LABEL org.opencontainers.image.description="Chaimir ClamAV middleware on the official ClamAV Alpine runtime"

COPY entrypoint.sh /usr/local/bin/chaimir-clamav-entrypoint
RUN mkdir -p /var/lib/clamav /run/clamav /tmp/clamav \
    && chown -R 100:101 /var/lib/clamav /run/clamav /tmp/clamav /usr/local/bin/chaimir-clamav-entrypoint \
    && chmod 0755 /usr/local/bin/chaimir-clamav-entrypoint

USER 100:101
EXPOSE 3310 7357
ENTRYPOINT ["/usr/local/bin/chaimir-clamav-entrypoint"]
