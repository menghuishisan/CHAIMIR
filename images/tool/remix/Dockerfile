# 本文件定义 Remix Solidity IDE 工具镜像。
FROM remixproject/remix-ide:remix_live

LABEL org.opencontainers.image.title="tool/remix"
LABEL org.opencontainers.image.description="Chaimir Remix Solidity IDE image"

USER root

# Remix 使用 nginx 提供静态 IDE,运行期必须以非 root 写入 pid 与临时目录。
RUN mkdir -p /workspace /tmp/nginx/client_temp /tmp/nginx/proxy_temp /tmp/nginx/fastcgi_temp /tmp/nginx/uwsgi_temp /tmp/nginx/scgi_temp \
    && chown -R 1000:1000 /workspace /tmp/nginx \
    && sed -i 's|pid .*|pid /tmp/nginx.pid;|' /etc/nginx/nginx.conf \
    && sed -i 's|error_log  /var/log/nginx/error.log notice;|error_log /dev/stderr notice;|' /etc/nginx/nginx.conf \
    && sed -i 's|access_log  /var/log/nginx/access.log  main;|access_log /dev/stdout main;|' /etc/nginx/nginx.conf \
    && sed -i '/include \/etc\/nginx\/conf.d\/\*.conf;/i\    client_body_temp_path /tmp/nginx/client_temp;\n    proxy_temp_path /tmp/nginx/proxy_temp;\n    fastcgi_temp_path /tmp/nginx/fastcgi_temp;\n    uwsgi_temp_path /tmp/nginx/uwsgi_temp;\n    scgi_temp_path /tmp/nginx/scgi_temp;' /etc/nginx/nginx.conf \
    && sed -i 's|listen       80;|listen       8080;|' /etc/nginx/conf.d/default.conf
COPY entrypoint.sh /usr/local/bin/chaimir-remix-entrypoint
RUN chmod +x /usr/local/bin/chaimir-remix-entrypoint

USER 1000:1000
WORKDIR /workspace
EXPOSE 8080
ENTRYPOINT ["/usr/local/bin/chaimir-remix-entrypoint"]
