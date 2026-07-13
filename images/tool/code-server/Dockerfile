# 本文件定义 code-server 工具镜像,复用官方维护的 Web IDE 应用并收紧运行层。
FROM ghcr.io/coder/code-server:4.126.0@sha256:e1f03b5faaefd63ba6c7173a5290c6ec0526ac907f23acfe6bc949dd965279e4 AS upstream

FROM node:22-trixie-slim@sha256:4228fca437e45714a3ebd1d4ecd1dcc583cf79f5a940aa025e286b472d93b67c AS patched
COPY --from=upstream /usr/lib/code-server /usr/lib/code-server
COPY --from=upstream /usr/bin/code-server /usr/bin/code-server
RUN npm --prefix /usr/lib/code-server install --omit=dev --ignore-scripts --package-lock=false --no-audit --no-fund \
        basic-ftp@5.3.1 \
        minimatch@3.1.4 \
        path-to-regexp@8.4.0 \
        tar@7.5.11 \
        ws@8.21.0 \
    && npm --prefix /usr/lib/code-server/lib/vscode install --omit=dev --ignore-scripts --package-lock=false --no-audit --no-fund \
        picomatch@4.0.4 \
        tar-fs@2.1.4 \
        undici@7.28.0 \
    && test -x /usr/lib/code-server/bin/code-server

FROM cgr.dev/chainguard/wolfi-base@sha256:02dab76bd852a70556b5b2002195c8a5fdab77d323c433bf6642aab080489795

LABEL org.opencontainers.image.title="tool/code-server"
LABEL org.opencontainers.image.description="Chaimir code-server web IDE image"

RUN --mount=type=cache,target=/var/cache/apk,sharing=locked \
    packages='bash=5.3-r12 ca-certificates=20260413-r0 git=2.55.0-r3 git-lfs=3.7.1-r17 openssh-client=10.4_p1-r1 nodejs-24=24.18.0-r2 npm=12.0.1-r0' \
    && for attempt in 1 2 3 4 5 6 7 8; do \
        apk add --cache-dir /var/cache/apk --update-cache ${packages} && break; \
        if [ "${attempt}" -eq 8 ]; then exit 1; fi; \
        sleep $((attempt * 3)); \
    done \
    && addgroup -g 1000 coder \
    && adduser -D -u 1000 -G coder -h /home/coder -s /bin/bash coder \
    && mkdir -p /workspace /home/coder/.config /home/coder/.local/share/code-server \
    && chown -R 1000:1000 /workspace /home/coder

COPY --from=patched /usr/lib/code-server /usr/lib/code-server
COPY --from=patched /usr/bin/code-server /usr/bin/code-server
RUN rm -f /usr/lib/code-server/lib/node \
    && ln -s /usr/bin/node /usr/lib/code-server/lib/node

ENV HOME=/home/coder
USER 1000:1000
WORKDIR /workspace
EXPOSE 8080

# code-server 只监听容器内端口,外部访问由平台代理鉴权。
CMD ["code-server", "--bind-addr", "0.0.0.0:8080", "--auth", "none", "/workspace"]
