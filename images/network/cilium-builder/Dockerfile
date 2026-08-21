# 本文件提供构建 Cilium agent 与 generic operator 的受控 Alpine 构建基座。
ARG GO_BUILDER_IMAGE=invalid.invalid/base/go-builder:required

FROM ${GO_BUILDER_IMAGE} AS golang-dist
FROM quay.io/cilium/cilium-llvm:19.1.7-1785833026-d8383c5@sha256:80ae6ceb1a1556d5f65f7a086d873c4706ed548444c4d49d89cbeedf24f817d4 AS llvm-dist
FROM docker.io/library/alpine:3.23@sha256:fd791d74b68913cbb027c6546007b3f0d3bc45125f797758156952bc2d6daf40

ARG GO_MODULE_PROXY=https://proxy.golang.org|direct
ARG CILIUM_SOURCE_URL=https://github.com/cilium/cilium/archive/refs/tags/v1.20.1.tar.gz
ARG CILIUM_SOURCE_SHA256=6b133c7b5a6a037a15c8b5ce02d7fcf94e98d90d121fb76b712c79eda9d36731
ARG CILIUM_SOURCE_COMMIT=7d68cfb394f2960e10aa72e76d0d51e66c1b2ebc
ARG GOPS_SOURCE_URL=https://github.com/google/gops/archive/refs/tags/v0.3.29.tar.gz
ARG GOPS_SOURCE_SHA256=c4637684ac8efbcc0bc55faf64e3920a7764a9ae8f4580945084c1a5b8b2d051

USER root
RUN apk add --no-cache bash binutils ca-certificates ccache coreutils elfutils-dev gcc git linux-headers make musl-dev pkgconf python3 py3-jinja2 scapy \
    && test ! -e /usr/bin/pebble

COPY --from=golang-dist /usr/local/go /usr/local/go
COPY --from=llvm-dist /usr/local/bin/clang /usr/local/bin/llc /usr/local/bin/
COPY scripts/prepare-cilium-source.sh /usr/local/bin/prepare-cilium-source

ENV GOROOT=/usr/local/go \
    GOPATH=/go \
    GOCACHE=/tmp/go-cache \
    GOPROXY=${GO_MODULE_PROXY} \
    PATH=/usr/local/go/bin:/go/bin:/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin

ADD --checksum=sha256:6b133c7b5a6a037a15c8b5ce02d7fcf94e98d90d121fb76b712c79eda9d36731 ${CILIUM_SOURCE_URL} /tmp/cilium-source.tar.gz
ADD --checksum=sha256:c4637684ac8efbcc0bc55faf64e3920a7764a9ae8f4580945084c1a5b8b2d051 ${GOPS_SOURCE_URL} /tmp/gops-source.tar.gz
RUN mkdir -p /src/cilium /go /tmp/go-cache \
    && tar -xzf /tmp/cilium-source.tar.gz -C /src/cilium --strip-components=1 \
    && rm -f /tmp/cilium-source.tar.gz \
    && chmod 0755 /usr/local/bin/prepare-cilium-source \
    && /usr/local/bin/prepare-cilium-source /src/cilium ${CILIUM_SOURCE_COMMIT} \
    && mkdir -p /tmp/gops \
    && tar -xzf /tmp/gops-source.tar.gz -C /tmp/gops --strip-components=1 \
    && cd /tmp/gops \
    && CGO_ENABLED=0 go build -trimpath -ldflags='-s -w' -o /usr/bin/gops github.com/google/gops \
    && cd /src/cilium \
    && mkdir -p /.cache /go/bin \
    && chmod 0777 /.cache /go /go/bin /tmp/go-cache \
    && rm -rf /go/pkg/mod /root/.cache /tmp/gops /tmp/gops-source.tar.gz

WORKDIR /src/cilium
LABEL org.opencontainers.image.title="network/cilium-builder"
LABEL org.opencontainers.image.description="Cilium v1.20.1 Alpine build base with x/text v0.39.0 vendored"
