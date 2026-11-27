# infra/chainlink-node

> 更新：2026-11-27。机器字段以本目录 [manifest](manifest.yaml) 为准；构建、供应链、通用隔离和验收遵循[镜像规范](../../../docs/总-镜像与容器设计.md)。

## 镜像职责

Chainlink 预言机节点镜像。

## 合法 runtime / provider

- 必需非敏感配置：`gas_price_max_wei`、`vrf_gas_limit`，分别由统一 `deploy/config/chaimir.env` 的 `CHAINLINK_GAS_PRICE_MAX_WEI`、`CHAINLINK_VRF_GAS_LIMIT` 通过 CompositionSpec 参数注入；前者限制 EVM gas 价格，后者覆盖 VRF fulfill 的 gas 估算上限，均禁止依赖不适用的官方默认值。
- 链编号与 Gas 价格为正整数，VRF Gas 范围为 1–4294967295，与原生 uint32 一致；目录 schema 与入口同时校验。

## bindings 与 links

- 数据库 URL 由入口固定生成 `sslmode=require`，与平台 PostgreSQL TLS 契约一致。

## command / args 语义

- 入口把 `CHAINLINK_GAS_PRICE_MAX_WEI`、`CHAINLINK_VRF_GAS_LIMIT` 写入官方 Chainlink gas estimator 配置，并统一启用 FeedsManager、OCR2 与 P2P v2；6690 只承担内部节点通信。

## Secret / KMS

- 密钥来源：manifest 声明的数据库密码、节点凭据、API 凭据和 TLS 材料均由平台 Secret/KMS 注入；TLS 证书和私钥以 Base64 Secret 值注入后仅写入 runtime-state，绝不进入镜像层。Chainlink EVM oracle 钱包地址和余额属于组合生成的运行时材料，由受控 EVM 账户注资，不能写入镜像或仓库。
- 数据库 URL 由 Go 标准库编码，TOML 由上游已有库生成，密码不裁剪或限制为 URL 安全字符。API 凭据按原生两行格式保存，邮箱和密码禁止换行；TLS 材料在写入前检查主机名、有效期、服务端用途和密钥匹配。

## 持久化与隔离边界

- 原生 `RootDir`、配置、API 凭据及 TLS 文件统一写入 `/runtime-state/chainlink`；目录 0700、生成文件 0600。每次启动重读 Secret，交接原生节点前清除已转换的敏感环境变量及旧式 `ETH_URL`。

## readiness / health

- 探针读取运行期 TLS 证书，以受控 CA、固定主机映射和有界超时访问 `https://chainlink-node:6688/readyz`。K8s 不使用明文 HTTP 探针或只代表端口已监听的 TCP 探针。
- 镜像 healthcheck 与 readiness 使用同一项目脚本；没有 TLS 材料、证书不可读、HTTPS 服务未 ready 或证书主机不匹配时均失败。
