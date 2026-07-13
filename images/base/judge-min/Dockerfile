# 本文件定义判题执行器安全基座,供具体 judger 镜像继承。
ARG GO_BUILDER_IMAGE=invalid.invalid/base/go-builder:required

FROM ${GO_BUILDER_IMAGE} AS normalizer-build

COPY normalize_result.go /src/normalize_result.go
RUN go build -trimpath -ldflags "-s -w" -o /out/normalize-result /src/normalize_result.go

FROM alpine:3.23@sha256:fd791d74b68913cbb027c6546007b3f0d3bc45125f797758156952bc2d6daf40

LABEL org.opencontainers.image.title="base/judge-min"
LABEL org.opencontainers.image.description="Chaimir minimal judge runner base image"

# 安装证书和基础 shell 工具,满足判题执行器拉取受控依赖与执行脚本的最小需要。
USER root
COPY --from=normalizer-build /out/normalize-result /usr/local/bin/normalize-result
RUN apk upgrade --no-cache \
    && apk add --no-cache ca-certificates bash coreutils \
    && addgroup -S chaimir \
    && adduser -S -G chaimir -h /workspace chaimir \
    && chmod 0755 /usr/local/bin/normalize-result

WORKDIR /workspace
USER chaimir

# 基座镜像只做环境自检,不承担具体判题任务。
CMD ["bash", "-lc", "id -u && test -d /workspace && normalize-result --mode exit-code --exit-code 0 --source selftest | grep '\"passed\":true' >/dev/null"]
