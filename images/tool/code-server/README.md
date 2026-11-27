# tool/code-server

> 更新：2026-11-27。机器字段以本目录 [manifest](manifest.yaml) 为准；构建、供应链、通用隔离和验收遵循[镜像规范](../../../docs/总-镜像与容器设计.md)。

## 镜像职责

VS Code Web 编辑器镜像。
工具在只读根文件系统下运行,配置、运行数据及用户缓存分别由 manifest 声明的临时卷提供;缓存卷对应上游使用的 `/home/coder/.cache`,防止扩展宿主和首次编辑器加载尝试写入只读 HOME。临时卷随沙箱回收,代码仍由正式工作区持久化。
### 离线 Solidity 编辑
镜像内置 MIT 许可的 `tintinweb.vscode-solidity-language` 0.0.7,来源为 Open VSX 的固定版本 VSIX,解包前由 Docker `ADD --checksum` 校验 SHA-256 `cd932cfeb93d4deee1415b77f35b35c4ec402f82e516c230f95b7e6a96ef56c8`。扩展只贡献 `.sol` 语言识别、TextMate 语法、括号/缩进配置和主题,没有扩展执行入口或 npm 依赖,不在运行时下载编译器或向外发送源码。编译、部署和链上动作继续由明确选择的 runtime 能力完成,不在编辑器容器复制私有运行时。语言文件安装在只读内置扩展目录,不放入会被临时数据卷遮蔽的用户扩展目录。

## bindings 与 links

- 四项可选连接分别用于索引部署、索引进度、索引查询和 IPFS 内容上传。
- 教师在编排中明确选择目标；未选择时不增加依赖、地址或网络权限，只有一个候选也不自动连接。
- 选中后分别注入 `CHAIMIR_GRAPH_ADMIN_URL`、`CHAIMIR_GRAPH_STATUS_URL`、`CHAIMIR_GRAPH_QUERY_URL` 和 `CHAIMIR_IPFS_API_URL`，学生可在 IDE 终端通过已有 Node HTTP 能力调用服务的原生接口。subgraph 的已编译材料由工作区提供；本镜像不内置 Graph CLI，也不在运行时下载依赖。
- 选中的连接沿用端点、协议、能力和启动前置校验；重复或错误目标拒绝编排。只放行对应服务端口，不注入数据库密码、POI token 或链私有状态。

## 端口与服务

- 就绪路径：manifest的`tool.readiness_path=/healthz`,使用固定上游已有健康路由,不加载工作台首页。该声明进入migrate内置目录和新组合快照,旧冻结快照保持原值;Pod/EndpointSlice就绪只证明可接入,扩展宿主和真实编辑仍需新实例页面验收。

## 验收条件

- 选择四项连接后，在 IDE 终端上传 subgraph 材料、调用原生部署接口、读取索引进度并查询链上事件；撤销连接后，新快照不得保留旧地址和网络规则。另验证普通编辑环境不拉入索引器或 IPFS。
