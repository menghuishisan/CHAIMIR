# 本文件定义 Chaimir 自研教学链运行时镜像。
ARG GO_BUILDER_IMAGE=invalid.invalid/base/go-builder:required

FROM ${GO_BUILDER_IMAGE} AS build
WORKDIR /src/edu-chain
COPY edu_chain.go .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath -ldflags="-s -w" -o /out/chaimir-edu-chain ./edu_chain.go

FROM scratch

LABEL org.opencontainers.image.title="runtime/edu-chain"
LABEL org.opencontainers.image.description="Chaimir deterministic teaching chain runtime"

COPY --from=build --chown=1000:1000 /out/chaimir-edu-chain /usr/local/bin/chaimir-edu-chain

USER 1000:1000
WORKDIR /workspace
EXPOSE 8080
ENTRYPOINT ["/usr/local/bin/chaimir-edu-chain"]
