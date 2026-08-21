# network/cilium-builder

该构建基座使用已通过门禁的 Go 1.26.6 工具链、Alpine 3.23 构建层和固定 Cilium LLVM clang/llc，下载并校验 Cilium `v1.20.1` 发布源码，随后将 `golang.org/x/text` 更新至 `v0.39.0` 并重建 vendor。构建结束会清理 Go 模块下载缓存，避免把测试夹具或私钥样例带进镜像。agent 和 generic operator 只从这一份已修复源码构建，避免相同依赖树出现两套版本。
