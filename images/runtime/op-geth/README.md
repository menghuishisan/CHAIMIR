# runtime/op-geth

本镜像提供 Optimism L2 的执行层能力。平台适配入口会校验运行期 genesis/JWT，首次启动执行一次 `geth init`，随后固定开启 JSON-RPC 和带 JWT 的 Engine API；数据目录只写入 `runtime-state`，不接受 `--dev`、解锁账户或任意覆盖参数。

能力：`evm-json-rpc`、`evm-engine-api`、`op-geth-execution`。Engine API 只允许 OP Node 通过 NetworkPolicy 访问，JWT 只来自运行期 Secret。镜像固定到官方 OP Labs amd64 digest，工作负载不得使用可变 tag。
