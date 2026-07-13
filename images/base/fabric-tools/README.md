# base/fabric-tools

本镜像从固定 commit 和 SHA256 校验的 Hyperledger Fabric v2.5.16 源码构建 `peer`、`configtxgen`、`configtxlator`、`cryptogen`、`discover`、`ledgerutil` 与 `osnadmin`,供 `runtime/fabric` 和 `judger/testcase-fabric` 复用。

它不包含通用 shell、网络工具或运行时系统层;这些职责属于 `base/chain-tools` 或各消费镜像。独立基座避免两个 Fabric 镜像重复执行同一源码构建,也避免把 Fabric 专用二进制注入无关运行时。
