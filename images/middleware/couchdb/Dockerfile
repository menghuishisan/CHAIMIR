# 本文件定义 CouchDB 的安全运行镜像,基于 Wolfi 包保留 Fabric 富查询所需 HTTP API。
FROM cgr.dev/chainguard/wolfi-base@sha256:a31344ab2cb8618db84f535eec56f76f6178b142cb92cb2e48676cc2dcebea72

LABEL org.opencontainers.image.title="middleware/couchdb"
LABEL org.opencontainers.image.description="Chaimir CouchDB middleware on Wolfi"

RUN for attempt in 1 2 3 4 5 6 7 8; do \
        if apk add --no-cache --timeout 300 couchdb-3.3=3.3.3-r16 couchdb-3.3-compat=3.3.3-r16; then break; fi; \
        if [ "$attempt" -eq 8 ]; then echo "CouchDB package installation failed after 8 attempts" >&2; exit 1; fi; \
        sleep $((attempt * 3)); \
    done
COPY entrypoint.sh /usr/local/bin/chaimir-couchdb-entrypoint
RUN if ! grep -q '^couchdb:' /etc/group; then addgroup -g 5984 couchdb; fi \
    && if ! grep -q '^couchdb:' /etc/passwd; then adduser -D -H -u 5984 -G couchdb couchdb; fi \
    && mkdir -p /opt/couchdb/data /opt/couchdb/etc/local.d /run/couchdb \
    && chown -R 5984:5984 /opt/couchdb/data /opt/couchdb/etc/local.d /run/couchdb \
    && chmod 0755 /usr/local/bin/chaimir-couchdb-entrypoint

USER 5984:5984
WORKDIR /opt/couchdb
EXPOSE 5984 4369 9100
VOLUME ["/opt/couchdb/data"]
ENTRYPOINT ["/usr/local/bin/chaimir-couchdb-entrypoint"]
