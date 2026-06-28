# 本文件定义 Cosmos SDK/Tendermint 单链运行时镜像,从 Gaia 官方源码构建。
FROM debian:12-slim AS build

ARG GAIA_VERSION=v27.4.0
ARG GAIA_SOURCE_COMMIT=849a28da07aac0aab12adaec22ff6d6a39f20d47
ARG GAIA_SOURCE_SHA256=fcef7a05b2de7a0f25669cdb7d10a6f2715b654ad457fcb213ca8826b4ac617a
ARG GAIA_GO_VERSION=go1.25.11
ARG GAIA_GO_SHA256=34f14304e856893f4ba30c2cacfe93906e9de7915c5f6aaaf3a81cdccd7ba30b
ARG GO_MODULE_PROXY=
ENV GOROOT=/usr/local/go
ENV GOPATH=/go
ENV GOCACHE=/tmp/go-cache
ENV PATH=/usr/local/go/bin:/go/bin:/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin

ADD https://github.com/cosmos/gaia/archive/${GAIA_SOURCE_COMMIT}.tar.gz /tmp/gaia-source.tar.gz
ADD https://go.dev/dl/${GAIA_GO_VERSION}.linux-amd64.tar.gz /tmp/go-gaia.tar.gz
RUN apt-get -o Acquire::Retries=5 update \
    && apt-get -o Acquire::Retries=5 -y --fix-missing upgrade --no-install-recommends \
    && apt-get -o Acquire::Retries=5 -y --fix-missing install --no-install-recommends ca-certificates build-essential \
    && echo "${GAIA_SOURCE_SHA256}  /tmp/gaia-source.tar.gz" | sha256sum -c - \
    && echo "${GAIA_GO_SHA256}  /tmp/go-gaia.tar.gz" | sha256sum -c - \
    && tar -C /usr/local -xzf /tmp/go-gaia.tar.gz \
    && groupadd --gid 1000 chaimir \
    && useradd --uid 1000 --gid 1000 --home-dir /src --create-home --shell /usr/sbin/nologin chaimir \
    && mkdir -p /go /tmp/go-cache /out \
    && tar -C /src --strip-components=1 -xzf /tmp/gaia-source.tar.gz \
    && chown -R 1000:1000 /src /go /tmp/go-cache /out \
    && rm -f /tmp/go-gaia.tar.gz /tmp/gaia-source.tar.gz \
    && rm -rf /var/lib/apt/lists/*
USER 1000:1000
# 固定官方 tag,并升级 Trivy 已给出 fixed version 的 Go 依赖。
WORKDIR /src
RUN if [ -n "${GO_MODULE_PROXY}" ]; then go env -w GOPROXY="${GO_MODULE_PROXY}"; fi \
    && go get \
        github.com/ethereum/go-ethereum@v1.16.9 \
        github.com/hashicorp/go-getter@v1.8.6 \
        go.opentelemetry.io/otel@v1.43.0 \
        go.opentelemetry.io/otel/sdk@v1.43.0 \
        github.com/shamaton/msgpack/v2@v2.4.1 \
        golang.org/x/crypto@v0.52.0 \
        golang.org/x/net@v0.55.0 \
    && go mod tidy \
    && CGO_CFLAGS="-D__BLST_PORTABLE__" go build -trimpath -tags "netgo" -ldflags "-s -w" -o /out/gaiad ./cmd/gaiad \
    && find /go/pkg/mod -name "libwasmvm.x86_64.so" -type f -print -exec cp {} /out/libwasmvm.x86_64.so \;

FROM debian:13.5-slim AS runtime-libs
RUN apt-get -o Acquire::Retries=3 update \
    && apt-get -o Acquire::Retries=3 -y install --no-install-recommends ca-certificates libgcc-s1 \
    && rm -rf /var/lib/apt/lists/*

FROM alpine:3.23 AS runtime-dirs
RUN mkdir -p /workspace /runtime-state/cosmos /tmp \
    && chown -R 1000:1000 /workspace /runtime-state /tmp

FROM scratch

LABEL org.opencontainers.image.title="runtime/cosmos"
LABEL org.opencontainers.image.description="Chaimir Cosmos SDK local chain runtime built from official Gaia source"

COPY --from=runtime-libs /lib/x86_64-linux-gnu/ /lib/x86_64-linux-gnu/
COPY --from=runtime-libs /lib64/ /lib64/
COPY --from=runtime-libs /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/ca-certificates.crt
COPY --from=runtime-dirs --chown=1000:1000 /workspace /workspace
COPY --from=runtime-dirs --chown=1000:1000 /runtime-state /runtime-state
COPY --from=runtime-dirs --chown=1000:1000 /tmp /tmp
COPY --from=build --chown=1000:1000 /out/gaiad /usr/local/bin/gaiad
COPY --from=build --chown=1000:1000 /out/libwasmvm.x86_64.so /usr/local/lib/libwasmvm.x86_64.so

ENV LD_LIBRARY_PATH=/usr/local/lib
ENV HOME=/workspace
ENV TMPDIR=/tmp
USER 1000:1000
WORKDIR /workspace
EXPOSE 26657 1317
ENTRYPOINT ["/usr/local/bin/gaiad"]
CMD ["start", "--home", "/runtime-state/cosmos"]
