# 本文件定义静态安全扫描判题镜像,使用 Slither 与官方 solc 进行 Solidity 安全判题。
ARG JUDGE_MIN_IMAGE

FROM ${JUDGE_MIN_IMAGE} AS judge-base

FROM alpine:3.23

ARG SOLC_VERSION=0.8.26
ARG SOLC_PATH=solc-linux-amd64-v0.8.26+commit.8a97fa7a
ARG SOLC_SHA256=d5f23436f443edb85d8e76906d12f0a86ce0490e7663a9e608efeb7a93f149ef

LABEL org.opencontainers.image.title="judger/static-scan"
LABEL org.opencontainers.image.description="Chaimir static scan judger image"

USER root
ADD https://binaries.soliditylang.org/linux-amd64/${SOLC_PATH} /usr/local/bin/solc-${SOLC_VERSION}
RUN apk upgrade --no-cache \
    && apk add --no-cache bash ca-certificates gcompat git libstdc++ py3-pip python3 \
    && echo "${SOLC_SHA256}  /usr/local/bin/solc-${SOLC_VERSION}" | sha256sum -c - \
    && chmod 0755 /usr/local/bin/solc-${SOLC_VERSION} \
    && ln -s /usr/local/bin/solc-${SOLC_VERSION} /usr/local/bin/solc \
    && python3 -m venv /opt/slither \
    && /opt/slither/bin/pip install --no-cache-dir --upgrade \
        "pip>=25.3" \
        "setuptools>=80.0.0" \
        "wheel>=0.46.0" \
        "slither-analyzer==0.11.3" \
    && ln -s /opt/slither/bin/slither /usr/local/bin/slither \
    && slither --version >/dev/null \
    && solc --version >/dev/null \
    && addgroup -S -g 1000 chaimir \
    && adduser -S -u 1000 -G chaimir -h /judge -s /sbin/nologin chaimir \
    && mkdir -p /judge /tmp/slither \
    && chown -R 1000:1000 /judge /tmp/slither

COPY run_static_scan.sh /usr/local/bin/run-static-scan
COPY --from=judge-base /usr/local/bin/normalize-result /usr/local/bin/normalize-result
RUN chmod +x /usr/local/bin/run-static-scan /usr/local/bin/normalize-result

USER 1000:1000
ENV HOME=/judge
ENV TMPDIR=/tmp/slither
ENV PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin:/opt/slither/bin
WORKDIR /judge
ENTRYPOINT ["run-static-scan"]
