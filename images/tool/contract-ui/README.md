# tool/contract-ui

读取 EVM ABI 并生成调用数据，供实验工作台交给当前链运行时执行。工具本身不连接节点、不保管账户，也不广播交易。

- `GET /`：交互页面；`GET /healthz`：就绪检查。
- `POST /abi/methods`：输入 ABI，返回方法名、规范签名、状态可变性和参数类型。
- `POST /call-data`：输入 `abi`、`method` 和 `args` 数组，返回规范签名与 calldata。
- 方法可使用唯一函数名；重载方法必须使用规范签名，例如 `set(uint256)`。不按参数试探或猜选重载。
- 支持基础类型、定长/动态数组、嵌套 tuple 与 tuple 数组。二进制参数使用 `0x` 十六进制字符串，大整数可使用十进制或十六进制字符串，tuple 使用按字段顺序排列的数组。非法类型、参数数量、数值范围和编码必须显式失败。

ABI 类型与编码复用镜像已固定的 eth-utils 和 eth-abi，不自建 ABI 编码器。

页面请求使用工具入口的相对路径，保留实验或竞赛网关前缀及其授权范围。

运行配置为 `CHAIMIR_IMAGE_PORT`、`CHAIMIR_MAX_BODY_BYTES`、`CHAIMIR_HTTP_TIMEOUT_SECONDS`，均须为正整数。端口、资源、供应链、挂载和隔离声明以 `manifest.yaml` 为准；容器保持非 root、只读根文件系统及默认拒绝网络。

验收先执行 manifest 自检，再在真实组合中用生成的调用数据完成链调用，并验证错误输入和资源回收。纯函数检查不能替代镜像、链与浏览器验收。
