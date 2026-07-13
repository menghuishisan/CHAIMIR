# 本文件构建钱包模拟器工具镜像。
FROM cgr.dev/chainguard/wolfi-base@sha256:02dab76bd852a70556b5b2002195c8a5fdab77d323c433bf6642aab080489795

LABEL org.opencontainers.image.title="tool/wallet-sim"
LABEL org.opencontainers.image.description="Chaimir wallet simulator tool image"

RUN --mount=type=cache,target=/var/cache/apk,sharing=locked \
    packages='python-3.13 py3.13-pip' \
    && for attempt in 1 2 3 4 5 6 7 8; do \
        apk add --cache-dir /var/cache/apk --update-cache ${packages} && break; \
        if [ "${attempt}" -eq 8 ]; then exit 1; fi; \
        sleep $((attempt * 3)); \
    done \
    && python3.13 -m venv /opt/wallet-sim \
    && /opt/wallet-sim/bin/pip install --no-cache-dir "eth-account==0.13.7" \
    && addgroup -g 1000 chaimir \
    && adduser -D -u 1000 -G chaimir -h /app -s /sbin/nologin chaimir

WORKDIR /app
COPY wallet_sim.py /app/wallet_sim.py

USER 1000:1000
ENV PATH=/opt/wallet-sim/bin:/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin
EXPOSE 8080
CMD ["python3.13", "/app/wallet_sim.py"]
