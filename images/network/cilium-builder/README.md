# network/cilium-builder

该构建基座下载并校验 Cilium `v1.20.0` 发布源码，随后将 `golang.org/x/text` 更新至 `v0.39.0` 并重建 vendor。agent 和 generic operator 只从这一份已修复源码构建，避免相同依赖树出现两套版本。
