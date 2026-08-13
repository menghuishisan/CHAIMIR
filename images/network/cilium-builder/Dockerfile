# 本文件提供构建 Cilium v1.20.0 agent 与 generic operator 的受控构建基座。
ARG CILIUM_RUNTIME_IMAGE=invalid.invalid/network/cilium-runtime:required

FROM docker.io/library/golang:1.26.5@sha256:3aff6657219a4d9c14e27fb1d8976c49c29fddb70ba835014f477e1c70636647 AS golang-dist
FROM ${CILIUM_RUNTIME_IMAGE}

ARG GO_MODULE_PROXY=https://proxy.golang.org|direct
ARG CILIUM_SOURCE_URL=https://github.com/cilium/cilium/archive/refs/tags/v1.20.0.tar.gz
ARG CILIUM_SOURCE_SHA256=07d7dff04f749ef26d6244ecd9affd83bde106a1eb8472d0d9aef636df01b91d
ARG CILIUM_SOURCE_COMMIT=450c53145dc0a872ae20dfb05598c94ef55a71e2

USER root
RUN apt-get update \
    && DEBIAN_FRONTEND=noninteractive apt-get install -y --no-install-recommends \
        binutils ccache coreutils gcc git libc6-dev libelf-dev make pkg-config python3-jinja2 python3-scapy \
    && apt-get clean \
    && rm -rf /var/lib/apt/lists/* /tmp/* /var/tmp/* \
    && test ! -e /usr/bin/pebble

COPY --from=golang-dist /usr/local/go /usr/local/go
COPY scripts/prepare-cilium-source.sh /usr/local/bin/prepare-cilium-source

ENV GOROOT=/usr/local/go \
    GOPATH=/go \
    GOCACHE=/tmp/go-cache \
    GOPROXY=${GO_MODULE_PROXY} \
    PATH=/usr/local/go/bin:/go/bin:/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin

ADD --checksum=sha256:07d7dff04f749ef26d6244ecd9affd83bde106a1eb8472d0d9aef636df01b91d ${CILIUM_SOURCE_URL} /tmp/cilium-source.tar.gz
RUN mkdir -p /src/cilium /go /tmp/go-cache \
    && tar -xzf /tmp/cilium-source.tar.gz -C /src/cilium --strip-components=1 \
    && rm -f /tmp/cilium-source.tar.gz \
    && chmod 0755 /usr/local/bin/prepare-cilium-source \
    && /usr/local/bin/prepare-cilium-source /src/cilium ${CILIUM_SOURCE_COMMIT} \
    && mkdir -p /.cache /go/bin \
    && chmod 0777 /.cache /go /go/bin /tmp/go-cache

WORKDIR /src/cilium
LABEL org.opencontainers.image.title="network/cilium-builder"
LABEL org.opencontainers.image.description="Cilium v1.20.0 build base with x/text v0.39.0 vendored"
