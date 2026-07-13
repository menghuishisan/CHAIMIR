# 本文件定义 noVNC 图形桌面工具镜像。
FROM alpine:3.23@sha256:fd791d74b68913cbb027c6546007b3f0d3bc45125f797758156952bc2d6daf40

LABEL org.opencontainers.image.title="tool/novnc-desktop"
LABEL org.opencontainers.image.description="Chaimir noVNC desktop tool image"

RUN apk add --no-cache \
        bash \
        ca-certificates \
        fluxbox \
        nginx \
        openbox \
        supervisor \
        tini \
        websockify \
        x11vnc \
        xterm \
        xkeyboard-config \
        xorg-server-common \
        xvfb \
        novnc \
    && addgroup -g 1000 chaimir \
    && adduser -D -u 1000 -G chaimir -h /workspace -s /sbin/nologin chaimir \
    && mkdir -p /workspace /tmp/novnc /tmp/.X11-unix \
    && chown -R 1000:1000 /workspace /tmp/novnc \
    && chmod 1777 /tmp/.X11-unix

COPY supervisord.conf /etc/supervisord.conf
RUN chmod 0644 /etc/supervisord.conf

ENV USER=chaimir
ENV HOME=/workspace
ENV DISPLAY=:1

USER 1000:1000
WORKDIR /workspace
EXPOSE 6080
ENTRYPOINT ["/sbin/tini", "--"]
CMD ["supervisord", "-c", "/etc/supervisord.conf"]
