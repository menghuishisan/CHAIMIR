# runtime/op-geth

本镜像提供 Optimism L2 的执行层能力。镜像携带经 manifest 声明的最小教学创世文件，平台适配入口仍会校验运行期 genesis/JWT，首次启动执行一次 `geth init`，随后固定开启 JSON-RPC 和带 JWT 的 Engine API；数据目录只写入 `runtime-state`，不接受 `--dev`、解锁账户或任意覆盖参数。需要真实 OP Stack 网络时，组合必须显式提供与 rollup 配置匹配的只读 genesis 材料，不得把教学创世文件冒充生产 L2 状态。

能力：`evm-json-rpc`、`evm-engine-api`、`op-geth-execution`。Engine API 只允许 OP Node 通过 NetworkPolicy 访问，JWT 只来自运行期 Secret。镜像固定到官方 OP Labs amd64 digest，工作负载不得使用可变 tag。
