# 本文件定义 ingress-nginx 控制器安全运行镜像,基于 Wolfi 维护包保留 Kubernetes Ingress 能力。
FROM cgr.dev/chainguard/wolfi-base@sha256:fdcd31a2db35958c251ea22e80cda72a8222228114e736ec7dd9c94452a2dc51

LABEL org.opencontainers.image.title="ingress/ingress-nginx"
LABEL org.opencontainers.image.description="Chaimir ingress-nginx controller on Wolfi"

# Wolfi Lua 包使用系统库目录,显式声明搜索路径以匹配控制器生成的 Nginx 配置。
ENV LUA_PATH="/usr/lib/lua/?.lua;/usr/share/lua/?.lua;;" \
    LUA_CPATH="/usr/lib/lua/?.so;;"

# 固定安装已包含安全修复的控制器与完整兼容运行时,网络抖动只在该步骤内重试。
RUN for attempt in 1 2 3 4 5 6 7 8 9 10; do \
        if apk add --no-cache --timeout 300 ingress-nginx-controller-1.15=1.15.10-r2 ingress-nginx-controller-compat-1.15=1.15.10-r2; then break; fi; \
        if [ "$attempt" -eq 10 ]; then echo "apk package installation failed after 10 attempts" >&2; exit 1; fi; \
        sleep 5; \
    done

# 控制器启动时会生成 TLS、遥测和 Nginx 状态文件,按上游 UID/GID 完成最小目录授权后再以运行身份校验。
RUN if ! grep -q '^www-data:' /etc/group; then addgroup -g 82 www-data; fi \
    && if ! grep -q '^www-data:' /etc/passwd; then adduser -D -H -s /sbin/nologin -u 101 -G www-data www-data; fi \
    && mkdir -p /etc/nginx /etc/ingress-controller/ssl /etc/ingress-controller/telemetry /tmp/nginx /var/lib/nginx /var/log/nginx \
    && chown -R 101:82 /etc/nginx /etc/ingress-controller /tmp/nginx /var/lib/nginx /var/log/nginx \
    && su -s /bin/sh -c 'nginx -t' www-data \
    && rm -f /tmp/nginx/nginx.pid \
    && test ! -e /tmp/nginx/nginx.pid

USER 101:82
WORKDIR /etc/nginx
EXPOSE 80 443 8443 10254
ENTRYPOINT ["/nginx-ingress-controller"]
