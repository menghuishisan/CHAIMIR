# 本文件定义 Remix Solidity IDE 工具镜像。
ARG GO_BUILDER_IMAGE=invalid.invalid/base/go-builder:required

FROM ${GO_BUILDER_IMAGE} AS server-build
WORKDIR /src/remix-server
COPY server.go .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath -ldflags="-s -w" -o /out/remix-static-server ./server.go

FROM remixproject/remix-ide:remix_live@sha256:0dc2f0c50a7e34d8d57bd49c38a6f557be8823f2869bd7891da75be6bf9dbbe0 AS remix-upstream

FROM gcr.io/distroless/static-debian12:nonroot@sha256:b7bb25d9f7c31d2bdd1982feb4dafcaf137703c7075dbe2febb41c24212b946f

LABEL org.opencontainers.image.title="tool/remix"
LABEL org.opencontainers.image.description="Chaimir Remix Solidity IDE image"

COPY --from=server-build /out/remix-static-server /usr/local/bin/remix-static-server
COPY --from=remix-upstream /usr/share/nginx/html /usr/share/remix

USER 1000:1000
WORKDIR /workspace
EXPOSE 8080
ENTRYPOINT ["/usr/local/bin/remix-static-server"]
