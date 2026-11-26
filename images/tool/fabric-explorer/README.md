# tool/fabric-explorer

通过组合注入的 peer、orderer 和组织身份提供 Fabric 浏览操作，全部链操作复用镜像固定版本的 peer CLI。

- 页面提供通道列表、区块摘要、链码只读查询和交易提交；API 使用当前工具入口的相对路径，支持实验与竞赛网关。
- GET /api/channels 读取已加入通道；GET /api/block?channel=&number= 返回区块字节数和 SHA-256，以流式读取计算摘要。
- POST /api/chaincode/query 和 POST /api/chaincode/submit 接收 channel、name、fcn、args（字符串数组）；提交等待交易事件，失败不能返回成功。请求不能改变 peer 或 orderer 地址。
- GET /healthz 通过实际通道查询验证 peer 可用；不以 CLI 版本或静态页面响应代替网络就绪。

运行参数、端点、TLS 材料路径和挂载以 manifest.yaml 为准。CHAIMIR_FABRIC_OUTPUT_MAX_BYTES 限制每次 CLI 的 stdout/stderr 字节数，超限明确失败；CHAIMIR_HTTP_TIMEOUT_SECONDS 限制请求及 CLI 执行时间。区块文件仅在临时目录使用，请求结束删除。

只读挂载所属 Fabric 实例的 runtime-state，以其组织身份访问显式绑定的 peer/orderer；不挂载学生工作区，不接受外部节点或用户密钥路径。容器非 root、根文件系统只读，网络默认拒绝。

验收须完成真实组合中的通道、区块、链码查询及提交，并检查失败、超限、代理页面和临时资源清理。CLI 替身或健康检查不能替代链联动与浏览器验收。
