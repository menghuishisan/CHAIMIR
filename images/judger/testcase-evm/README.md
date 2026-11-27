# judger/testcase-evm

> 更新：2026-11-27。机器字段以本目录 [manifest](manifest.yaml) 为准；构建、供应链、通用隔离和验收遵循[镜像规范](../../../docs/总-镜像与容器设计.md)。

## 镜像职责

EVM Hardhat/Foundry 测试用例判题镜像。

## 初始化材料与运行时注入

- 本判题器的 `genesis_ref` 只能引用 `runtime/evm-foundry` manifest 已声明的受控内置自检 genesis；该引用是组合快照中的运行时材料绑定，不是判题镜像自行读取或生成创世文件。
- 其他 genesis、rollup、checkpoint、Hyperlane、Chainlink、合约或 subgraph 材料不在镜像内，必须由平台受控对象存储、初始化资产和 CompositionSpec 注入，并校验来源及 SHA-256；缺失时严格失败。

## 离线编译器与发布一致性

- `toolchain.compiler_versions` 声明的每个编译器都必须由固定摘要的 `runtime/evm-hardhat` 复制到镜像内;执行时在可写`/judge/home/.svm`建立指向这些固定文件的符号链接。编译器从只读镜像执行,工作卷无需执行权限。`/judge`挂空工作卷仍须可离线按项目版本编译,不得依赖被卷遮蔽的镜像层缓存、向只读`/judge-private`写HOME、运行时下载或回退版本。
- 新镜像必须经过唯一候选构建、回拉、扫描/签名验证和摘要晋升流程,再由平台目录自检及真实判分验证。文件清单和构建通过分别记录,均不能代替真实判分通过。
