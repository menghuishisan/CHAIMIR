# 工作区镜像提供公开文件与终端能力,并供独立非学生容器执行初始化脚本;不包含链适配器或运行时状态。
FROM alpine:3.23@sha256:fd791d74b68913cbb027c6546007b3f0d3bc45125f797758156952bc2d6daf40

RUN apk add --no-cache \
        bash tar coreutils findutils git ca-certificates \
        'jq=1.8.2-r0' \
        'curl=8.22.0-r0' \
        'libcurl=8.22.0-r0' \
        'libcrypto3=3.5.9-r0' \
        'libexpat=2.9.0-r0' \
        'libssl3=3.5.9-r0' \
    && addgroup -g 1000 -S chaimir \
    && adduser -u 1000 -G chaimir -h /workspace -s /bin/bash -S chaimir \
    && mkdir -p /workspace /runtime-state/workspace-home \
    && chown -R 1000:1000 /workspace /runtime-state

COPY images/base/chain-tools/chaimir-workspace /usr/local/bin/chaimir-workspace
RUN chmod 0755 /usr/local/bin/chaimir-workspace

WORKDIR /workspace
USER 1000:1000
CMD ["/usr/local/bin/chaimir-workspace", "selftest"]
