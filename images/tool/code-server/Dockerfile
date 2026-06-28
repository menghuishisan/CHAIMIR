# 本文件定义 code-server 工具镜像,复用官方维护的 Web IDE 应用并收紧运行层。
FROM ghcr.io/coder/code-server:4.126.0 AS upstream

FROM node:22-trixie-slim AS patched
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

FROM cgr.dev/chainguard/wolfi-base:latest

LABEL org.opencontainers.image.title="tool/code-server"
LABEL org.opencontainers.image.description="Chaimir code-server web IDE image"

RUN apk add --no-cache \
        bash \
        ca-certificates \
        git \
        git-lfs \
        openssh-client \
        nodejs-24 \
        npm \
    && addgroup -g 1000 coder \
    && adduser -D -u 1000 -G coder -h /home/coder -s /bin/bash coder \
    && mkdir -p /workspace /home/coder/.config /home/coder/.local/share/code-server \
    && chown -R 1000:1000 /workspace /home/coder

COPY --from=patched /usr/lib/code-server /usr/lib/code-server
COPY --from=patched /usr/bin/code-server /usr/bin/code-server
RUN rm -f /usr/lib/code-server/lib/node \
    && ln -s /usr/bin/node /usr/lib/code-server/lib/node

USER 1000:1000
WORKDIR /workspace
EXPOSE 8080

# code-server 只监听容器内端口,外部访问由平台代理鉴权。
CMD ["code-server", "--bind-addr", "0.0.0.0:8080", "--auth", "none", "/workspace"]
