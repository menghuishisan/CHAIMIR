# 本文件定义 Lighthouse Beacon 薄封装镜像,仅增加动态 binding 和 Secret 文件入口。
FROM sigp/lighthouse:8.2.2@sha256:cdb2a6e5eb6197091896b4dac5e607ce696e09355614b56cbe5fb40b1a4dac29

USER root
RUN (getent group 1000 >/dev/null || groupadd --gid 1000 chaimir) \
    && (id -u 1000 >/dev/null 2>&1 || useradd --uid 1000 --gid 1000 --home-dir /workspace --create-home --shell /usr/sbin/nologin chaimir) \
    && mkdir -p /runtime-state /workspace \
    && chown -R 1000:1000 /runtime-state /workspace

COPY --chmod=0755 entrypoint.sh /usr/local/bin/chaimir-beacon-entrypoint
USER 1000:1000
WORKDIR /workspace
EXPOSE 5052
ENTRYPOINT ["/usr/local/bin/chaimir-beacon-entrypoint"]
