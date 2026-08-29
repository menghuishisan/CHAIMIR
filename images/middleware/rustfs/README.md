# middleware/rustfs

本镜像提供平台统一的 `s3-object-storage` 能力，使用 RustFS 的 S3 兼容接口承载代码、附件、报告和备份对象。RustFS 使用 Apache-2.0 许可证，运行期以非 root 用户 `10001:10001` 写入 `/data`。

平台只依赖 S3 能力契约，不依赖 RustFS 的私有 API。后端统一使用 `OBJECT_STORAGE_*` 配置和 `s3://bucket/key` 对象引用；教师组合通过对象存储能力选择提供者，不能写死具体镜像名。

当前构建以固定的 `rustfs/rustfs:1.0.0-rc.3@sha256:800cf3f352a0a27e3275ca854a51f0027975d7acc7a0d52089a35bcc9fcbf0b5` 为基础，仅将 Alpine OpenSSL、libssl3、libcrypto3 更新到 `3.5.8-r0`，以消除 CVE-2026-14456。最终镜像必须通过项目 Trivy 0.72.0 高危/严重门禁、SBOM、Cosign 和正式 digest 锁晋升，并实际验证建桶、上传、读取、列举、服务端复制和删除。

生产环境不得暴露控制台或将对象存储地址返回给学生；所有文件访问继续通过平台统一文件服务和短时授权出口。
