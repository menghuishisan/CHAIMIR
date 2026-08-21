# ingress/kube-webhook-certgen

本目录固定 ingress-nginx admission webhook 的证书创建与修补工具。它只由官方 Helm chart 的短生命周期 Job 使用,通过专用 ServiceAccount 创建内部 TLS Secret 并修补 ValidatingWebhookConfiguration CA;镜像始终按上游 digest 拉取并进入离线交付清单。
