# 本文件定义 noVNC 图形桌面工具镜像。
FROM dorowu/ubuntu-desktop-lxde-vnc:focal

LABEL org.opencontainers.image.title="tool/novnc-desktop"
LABEL org.opencontainers.image.description="Chaimir noVNC desktop tool image"

USER root
RUN groupadd --gid 1000 chaimir \
    && useradd --uid 1000 --gid 1000 --home-dir /workspace --create-home --shell /usr/sbin/nologin chaimir \
    && mkdir -p /workspace/.config/pcmanfm/LXDE /tmp/nginx/client_temp /tmp/nginx/proxy_temp /tmp/nginx/fastcgi_temp /tmp/nginx/uwsgi_temp /tmp/nginx/scgi_temp \
    && ln -sf /usr/local/share/doro-lxde-wallpapers/desktop-items-0.conf /workspace/.config/pcmanfm/LXDE/desktop-items-0.conf \
    && sed -i -e 's|%USER%|chaimir|g' -e 's|%HOME%|/workspace|g' /etc/supervisor/conf.d/supervisord.conf \
    && sed -i '/^user=chaimir$/d' /etc/supervisor/conf.d/supervisord.conf \
    && sed -i 's|file=/var/run/supervisor.sock|file=/tmp/supervisor.sock|' /etc/supervisor/supervisord.conf \
    && sed -i 's|logfile=/var/log/supervisor/supervisord.log|logfile=/tmp/supervisord.log|' /etc/supervisor/supervisord.conf \
    && sed -i 's|pidfile=/var/run/supervisord.pid|pidfile=/tmp/supervisord.pid|' /etc/supervisor/supervisord.conf \
    && sed -i 's|childlogdir=/var/log/supervisor|childlogdir=/tmp|' /etc/supervisor/supervisord.conf \
    && sed -i 's|serverurl=unix:///var/run/supervisor.sock|serverurl=unix:///tmp/supervisor.sock|' /etc/supervisor/supervisord.conf \
    && sed -i "s|'/var/log/web.log'|'/tmp/web.log'|" /usr/local/lib/web/backend/run.py \
    && sed -i 's|pid .*|pid /tmp/nginx.pid;|' /etc/nginx/nginx.conf \
    && sed -i 's|access_log /var/log/nginx/access.log;|access_log /dev/stdout;|' /etc/nginx/nginx.conf \
    && sed -i 's|error_log /var/log/nginx/error.log;|error_log /dev/stderr;|' /etc/nginx/nginx.conf \
    && sed -i '/http {/a\	client_body_temp_path /tmp/nginx/client_temp;\n	proxy_temp_path /tmp/nginx/proxy_temp;\n	fastcgi_temp_path /tmp/nginx/fastcgi_temp;\n	uwsgi_temp_path /tmp/nginx/uwsgi_temp;\n	scgi_temp_path /tmp/nginx/scgi_temp;' /etc/nginx/nginx.conf \
    && sed -i 's|listen 80 default_server;|listen 8080 default_server;|' /etc/nginx/sites-enabled/default \
    && sed -i 's|listen \\[::\\]:80 default_server;|listen [::]:8080 default_server;|' /etc/nginx/sites-enabled/default \
    && touch /tmp/web.log \
    && chown -R 1000:1000 /workspace /tmp/nginx /tmp/web.log /var/log/supervisor /var/log/nginx

ENV USER=chaimir
ENV HOME=/workspace

USER 1000:1000
WORKDIR /workspace
EXPOSE 8080
ENTRYPOINT ["/bin/tini", "--"]
CMD ["supervisord", "-n", "-c", "/etc/supervisor/supervisord.conf"]
