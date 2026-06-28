# 本文件定义 CouchDB 的安全运行镜像,基于 Wolfi 包保留 Fabric 富查询所需 HTTP API。
FROM cgr.dev/chainguard/wolfi-base:latest

LABEL org.opencontainers.image.title="middleware/couchdb"
LABEL org.opencontainers.image.description="Chaimir CouchDB middleware on Wolfi"

RUN apk add --no-cache couchdb-3.3 couchdb-3.3-compat
COPY entrypoint.sh /usr/local/bin/chaimir-couchdb-entrypoint
RUN addgroup -g 5984 couchdb || true \
    && adduser -D -H -u 5984 -G couchdb couchdb || true \
    && mkdir -p /opt/couchdb/data /opt/couchdb/etc/local.d /run/couchdb \
    && chown -R 5984:5984 /opt/couchdb/data /opt/couchdb/etc/local.d /run/couchdb \
    && chmod 0755 /usr/local/bin/chaimir-couchdb-entrypoint

USER 5984:5984
WORKDIR /opt/couchdb
EXPOSE 5984 4369 9100
VOLUME ["/opt/couchdb/data"]
ENTRYPOINT ["/usr/local/bin/chaimir-couchdb-entrypoint"]
