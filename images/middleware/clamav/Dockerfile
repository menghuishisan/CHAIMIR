# 本文件定义 ClamAV clamd 的安全运行镜像,基于 Wolfi 包保留 INSTREAM 扫描能力。
FROM cgr.dev/chainguard/wolfi-base:latest

LABEL org.opencontainers.image.title="middleware/clamav"
LABEL org.opencontainers.image.description="Chaimir ClamAV middleware on Wolfi"

COPY entrypoint.sh /usr/local/bin/chaimir-clamav-entrypoint
RUN apk add --no-cache clamav-1.5-daemon clamav-1.5-scanner clamav-1.5-freshclam clamav-1.5-db su-exec \
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
