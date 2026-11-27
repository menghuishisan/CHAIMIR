# infra/grafana

> 更新：2026-11-27。机器字段以本目录 [manifest](manifest.yaml) 为准；构建、供应链、通用隔离和验收遵循[镜像规范](../../../docs/总-镜像与容器设计.md)。

## 镜像职责

Grafana 监控看板官方 distroless 镜像；核心服务从固定源码重建，Prometheus 数据源使用 Grafana 13.1.6 核心能力。运行时使用与源码重建二进制 ABI 匹配的、按 digest 锁定的 glibc distroless 基座。

## 初始化材料与运行时注入

- 服务源码使用项目登记的 `base/go-builder` 候选 digest 构建基座；不是运行时依赖，也不作为镜像内能力发布。
- Prometheus 数据源来自固定的 Grafana 13.1.6 官方源码核心实现；上游镜像只提供固定版本的静态资源和配置，外置 bundled 插件目录在组装阶段清空，不下载、重打包、重签或允许未签名的外置插件。
- 核心源码构建阶段统一锁定 `github.com/apache/thrift v0.24.0`、`github.com/grafana/tempo v2.10.3` 和 `google.golang.org/grpc v1.83.2`。Tempo 使用固定官方源码归档和 SHA-256；由于该上游 v2 标签的 go.mod 未声明 `/v2` 模块路径，builder 内对固定源码和 Grafana 引用做确定性的模块路径规范化后再编译，不使用旧版本兼容分支或未锁定依赖。

## 验收条件

- 安全门禁必须确认 Grafana 核心二进制及其随核心发布的数据源能力无 HIGH/CRITICAL；不能只检查 Grafana HTTP 探活。
