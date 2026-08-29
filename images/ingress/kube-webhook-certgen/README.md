# ingress/kube-webhook-certgen

该镜像从 ingress-nginx 的 kube-webhook-certgen 源码构建，使用固定 Go 1.26.6 和 distroless nonroot 运行层；构建时将 `golang.org/x/net` 固定到 `v0.56.0`、`golang.org/x/text` 固定到 `v0.39.0`，避免上游 v1.6.9 二进制中的已知高危依赖。

本目录固定 ingress-nginx admission webhook 的证书创建与修补工具。它只由官方 Helm chart 的短生命周期 Job 使用,通过专用 ServiceAccount 创建内部 TLS Secret 并修补 ValidatingWebhookConfiguration CA;镜像由固定源码和不可变运行层重建后进入离线交付清单。
