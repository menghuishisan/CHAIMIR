# 本文件定义 ingress-nginx 控制器安全运行镜像,基于 Wolfi 维护包保留 Kubernetes Ingress 能力。
FROM cgr.dev/chainguard/wolfi-base@sha256:02dab76bd852a70556b5b2002195c8a5fdab77d323c433bf6642aab080489795

LABEL org.opencontainers.image.title="ingress/ingress-nginx"
LABEL org.opencontainers.image.description="Chaimir ingress-nginx controller on Wolfi"

RUN apk add --no-cache ingress-nginx-controller-1.15=1.15.8-r7 ingress-nginx-controller-compat-1.15=1.15.8-r7 \
    && addgroup -g 101 nginx || true \
    && adduser -D -H -u 101 -G nginx nginx || true \
    && mkdir -p /etc/nginx /tmp/nginx /var/lib/nginx /var/log/nginx \
    && chown -R 101:101 /etc/nginx /tmp/nginx /var/lib/nginx /var/log/nginx

USER 101:101
WORKDIR /etc/nginx
EXPOSE 80 443 8443 10254
ENTRYPOINT ["/nginx-ingress-controller"]
