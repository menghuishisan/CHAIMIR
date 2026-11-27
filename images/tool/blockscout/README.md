# tool/blockscout

> 更新：2026-11-27。机器字段以本目录 [manifest](manifest.yaml) 为准；构建、供应链、通用隔离和验收遵循[镜像规范](../../../docs/总-镜像与容器设计.md)。

## 镜像职责

EVM 区块链浏览器镜像。

## bindings 与 links

- 数据库密码通过已有 jq 的 URI 编码进入原生 URL，保留空白、Unicode 和特殊字符；入口只接受 `ECTO_SSL_MODE=require`，连接串明确使用 `sslmode=require`。
- 固定上游源码中的整串 URL 解码和凭据正则由 `database-url.patch` 替换为已有 Ecto 解析器；保留环境变量优先级和 `search_path` 归一化，构建时补丁不匹配立即失败。
