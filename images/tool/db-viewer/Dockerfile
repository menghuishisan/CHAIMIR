# 本文件定义数据库可视化工具镜像,从 pgweb 官方源码重建单二进制运行时。
ARG GO_BUILDER_IMAGE=invalid.invalid/base/go-builder:required

FROM ${GO_BUILDER_IMAGE} AS build

ARG PGWEB_VERSION=v0.17.0
ARG PGWEB_COMMIT=6b0b0244d1aefd6971999b03481eeeaa4ec7cf55

RUN git clone --branch "${PGWEB_VERSION}" --depth 1 https://github.com/sosedoff/pgweb.git /src/pgweb
WORKDIR /src/pgweb
RUN test "$(git rev-parse HEAD)" = "${PGWEB_COMMIT}" \
    && go get \
        github.com/sirupsen/logrus@v1.9.4 \
        golang.org/x/crypto@v0.52.0 \
        golang.org/x/net@v0.55.0 \
    && go mod tidy \
    && CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath -ldflags="-s -w" -o /out/pgweb .

FROM gcr.io/distroless/static-debian12:nonroot@sha256:b7bb25d9f7c31d2bdd1982feb4dafcaf137703c7075dbe2febb41c24212b946f

LABEL org.opencontainers.image.title="tool/db-viewer"
LABEL org.opencontainers.image.description="Chaimir database viewer tool image"

COPY --from=build /out/pgweb /usr/local/bin/pgweb

USER 1000:1000
WORKDIR /workspace
EXPOSE 8081
CMD ["pgweb", "--bind=0.0.0.0", "--listen=8081"]
