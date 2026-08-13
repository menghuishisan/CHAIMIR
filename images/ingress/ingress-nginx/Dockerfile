# 本文件定义 ingress-nginx 控制器安全运行镜像,基于 Wolfi 维护包保留 Kubernetes Ingress 能力。
FROM cgr.dev/chainguard/wolfi-base@sha256:02dab76bd852a70556b5b2002195c8a5fdab77d323c433bf6642aab080489795

LABEL org.opencontainers.image.title="ingress/ingress-nginx"
LABEL org.opencontainers.image.description="Chaimir ingress-nginx controller on Wolfi"

RUN for attempt in 1 2 3 4 5 6 7 8 9 10; do \
        if apk add --no-cache --timeout 300 ingress-nginx-controller-1.15=1.15.10-r0 ingress-nginx-controller-compat-1.15=1.15.10-r0; then break; fi; \
        if [ "$attempt" -eq 10 ]; then echo "apk package installation failed after 10 attempts" >&2; exit 1; fi; \
        sleep 5; \
    done \
    && if ! grep -q '^nginx:' /etc/group; then addgroup -g 101 nginx; fi \
    && if ! grep -q '^nginx:' /etc/passwd; then adduser -D -H -u 101 -G nginx nginx; fi \
    && mkdir -p /etc/nginx /tmp/nginx /var/lib/nginx /var/log/nginx \
    && chown -R 101:101 /etc/nginx /tmp/nginx /var/lib/nginx /var/log/nginx

USER 101:101
WORKDIR /etc/nginx
EXPOSE 80 443 8443 10254
ENTRYPOINT ["/nginx-ingress-controller"]
