# 本文件定义 ClamAV clamd 的安全运行镜像,基于 Wolfi 包保留 INSTREAM 扫描能力。
FROM cgr.dev/chainguard/wolfi-base@sha256:02dab76bd852a70556b5b2002195c8a5fdab77d323c433bf6642aab080489795

LABEL org.opencontainers.image.title="middleware/clamav"
LABEL org.opencontainers.image.description="Chaimir ClamAV middleware on Wolfi"

COPY entrypoint.sh /usr/local/bin/chaimir-clamav-entrypoint
RUN --mount=type=cache,target=/var/cache/apk,sharing=locked \
    packages='clamav-1.5-daemon=1.5.2-r7 clamav-1.5-scanner=1.5.2-r7 clamav-1.5-freshclam=1.5.2-r7 clamav-1.5-db=1.5.2-r7 su-exec=0.3-r1' \
    && for attempt in 1 2 3 4 5 6 7 8; do \
        apk add --cache-dir /var/cache/apk --update-cache ${packages} && break; \
        if [ "${attempt}" -eq 8 ]; then exit 1; fi; \
        sleep $((attempt * 3)); \
    done \
    && addgroup -g 1000 chaimir \
    && adduser -D -H -u 1000 -G chaimir chaimir \
    && mkdir -p /var/lib/clamav /run/clamav /tmp/clamav \
    && chown -R 1000:1000 /var/lib/clamav /run/clamav /tmp/clamav \
    && chmod 0755 /usr/local/bin/chaimir-clamav-entrypoint \
    && sed -i 's/^Example/#Example/' /etc/clamav/clamd.conf \
    && sed -i 's|^#LocalSocket .*|LocalSocket /run/clamav/clamd.sock|' /etc/clamav/clamd.conf \
    && sed -i 's/^#TCPSocket .*/TCPSocket 3310/' /etc/clamav/clamd.conf \
    && sed -i 's/^#TCPAddr .*/TCPAddr 0.0.0.0/' /etc/clamav/clamd.conf \
    && sed -i 's|^#DatabaseDirectory .*|DatabaseDirectory /var/lib/clamav|' /etc/clamav/clamd.conf \
    && sed -i 's|^#TemporaryDirectory .*|TemporaryDirectory /tmp/clamav|' /etc/clamav/clamd.conf \
    && sed -i 's/^#Foreground .*/Foreground yes/' /etc/clamav/clamd.conf \
    && sed -i 's|^LogFile .*|#LogFile disabled for container stdout|' /etc/clamav/clamd.conf \
    && sed -i 's/^#User .*/User chaimir/' /etc/clamav/clamd.conf \
    && printf '%s\n' \
      'DatabaseDirectory /var/lib/clamav' \
      'UpdateLogFile /tmp/clamav/freshclam.log' \
      'LogTime yes' \
      'Foreground yes' \
      'DatabaseMirror database.clamav.net' \
      > /etc/clamav/freshclam.conf

USER 1000:1000
EXPOSE 3310 7357
ENTRYPOINT ["/usr/local/bin/chaimir-clamav-entrypoint"]
