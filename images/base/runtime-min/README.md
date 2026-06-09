# base/runtime-min

最小运行时基座镜像,用于纯静态二进制或无需 shell 的生产服务镜像。

本镜像直接继承 distroless nonroot 基座,不安装调试工具,降低运行面和供应链风险。
