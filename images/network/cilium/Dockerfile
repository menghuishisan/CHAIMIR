# 本文件从统一的已修复 Cilium 源码构建 agent，并使用 Harbor network 组件作为全部镜像依赖。
ARG CILIUM_BUILDER_IMAGE=invalid.invalid/network/cilium-builder:required
ARG CILIUM_RUNTIME_IMAGE=invalid.invalid/network/cilium-runtime:required
ARG CILIUM_ENVOY_IMAGE=invalid.invalid/network/cilium-envoy:required

FROM ${CILIUM_BUILDER_IMAGE} AS builder

ARG TARGETOS
ARG TARGETARCH
WORKDIR /src/cilium
RUN --mount=type=cache,target=/go/pkg \
    --mount=type=cache,target=/tmp/go-cache \
    make GOARCH="${TARGETARCH}" DESTDIR="/tmp/install/${TARGETOS}/${TARGETARCH}" PKG_BUILD=1 \
        build-container install-container-binary install-bash-completion licenses-all \
    && mv LICENSE.all "/tmp/install/${TARGETOS}/${TARGETARCH}/LICENSE.all" \
    && cp images/cilium/init-container.sh "/tmp/install/${TARGETOS}/${TARGETARCH}/" \
    && cp plugins/cilium-cni/install-plugin.sh plugins/cilium-cni/cni-uninstall.sh "/tmp/install/${TARGETOS}/${TARGETARCH}/"

FROM ${CILIUM_ENVOY_IMAGE} AS envoy
FROM ${CILIUM_RUNTIME_IMAGE}

ARG TARGETOS
ARG TARGETARCH
LABEL org.opencontainers.image.title="network/cilium"
LABEL org.opencontainers.image.description="Cilium v1.20.0 agent rebuilt from patched official source"

RUN echo '. /etc/profile.d/bash_completion.sh' >> /etc/bash.bashrc
COPY --from=envoy /usr/bin/cilium-envoy /usr/bin/cilium-envoy-starter /usr/bin/
COPY --from=builder /tmp/install/${TARGETOS}/${TARGETARCH}/ /

ENV HUBBLE_SERVER=unix:///var/run/cilium/hubble.sock
WORKDIR /home/cilium
CMD ["/usr/bin/cilium-dbg"]
