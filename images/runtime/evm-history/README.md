# runtime/evm-history

固定历史区块的完整只读 EVM 状态提供者，复用 `runtime/evm-geth` 内由同版本官方源码编译的 `chaimir-evm-history`。不启动 Geth 节点、同步网络或暴露写操作；不是可部署交易的开发链。历史库为 hash state scheme，支持 LevelDB 和 Pebble；path scheme 或 Anvil dump/RPC 缓存不能充当完整历史库。

受控材料准备后，把已正常关闭的 Geth 数据库放入本目录的 `snapshot/chaindata/`，可包含 `ancient/`；不放入 keystore、JWT、配置和私钥。`snapshot/snapshot.json` 使用以下字段：`schema_version` 为 1，`engine` 为 `leveldb` 或 `pebble`，`block_number`、`chain_id` 为规范十进制字符串，`block_hash`、`state_root` 为完整 0x 哈希，`max_bytes` 为材料全部文件总容量上限（1 到 1 TiB）。该区块、前 256 个祖先区块与创世链配置必须可读，目标状态的全部账户、存储和字节码必须完整。真实材料目录被 Git 忽略；不得提交历史数据库、密钥或用不完整示例替代。

使用标准 `images/build-images.ps1` 构建；它从候选/正式 digest 锁解析 `EVM_GETH_IMAGE`。Dockerfile 的 `--verify` 流式遍历文件容量、全部状态 trie、存储 trie 和字节码，核验节点和代码哈希；缺材料或校验失败中止构建。冻结块和前 256 个祖先块须校验实际块头的编号、哈希，以及交易、叔块和提款列表与块头承诺的一致性；RPC 读取其他可用祖先块时执行同一校验，不能仅信任数据库索引。晋升仍须扫描、SBOM、签名和不可变 digest 锁，不能使用本地 tag。

组合参数 `history_block_number/history_block_hash/history_state_root/history_chain_id` 必须与镜像材料一致；M2 经 schema 注入环境变量。仅独立接入自检可省略身份参数，服务此时仍固定为镜像内历史身份。`--check` 检查材料身份和基本读取，供节点预拉取；不会重复全库遍历。数据库以 read-only 打开，镜像层不复制到 runtime-state。RPC 只提供链身份、固定块与祖先块，以及固定状态的余额、nonce、代码和存储读取，明确拒绝写入和其他历史状态查询。

`eth_getStorageAt` 的槽地址沿用同版本 Geth 解码约定：最多 32 字节的十六进制，接受 quantity、补零的完整槽地址、奇数位及可选 `0x/0X` 前缀，空十六进制表示零槽。不得用 quantity 的禁止前导零规则拒绝官方 `ethclient.StorageAt/StorageAtHash` 发送的 `common.Hash`；超长、非法十六进制和非冻结状态仍拒绝。

工作区以私有 RPC link 连接本提供者后执行原生 Anvil fork；链身份必须握手一致。发布前实测完整历史库镜像的 imagefs 占用、预拉取耗时、多个沙箱并发只读查询、重建及离线业务动作。受控小数据库测试不能替代这些验收。
