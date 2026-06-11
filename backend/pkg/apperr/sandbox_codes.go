// apperr sandbox_codes 文件定义 M2 沙箱引擎 21xxx/22xxx/23xxx/24xxx 错误码。
package apperr

const (
	// CodeSandboxRuntimeNotFound 表示运行时不存在。
	CodeSandboxRuntimeNotFound = "21001"
	// CodeSandboxRuntimeUnavailable 表示运行时未通过自检、停用或暂不可调度。
	CodeSandboxRuntimeUnavailable = "21002"
	// CodeSandboxRuntimeConfigInvalid 表示运行时或适配器配置非法。
	CodeSandboxRuntimeConfigInvalid = "21003"
	// CodeSandboxRuntimeImageNotFound 表示运行时镜像不存在。
	CodeSandboxRuntimeImageNotFound = "21004"
	// CodeSandboxSelftestFailed 表示运行时接入即测失败。
	CodeSandboxSelftestFailed = "21005"
	// CodeSandboxImagePrepullFailed 表示镜像预拉取失败。
	CodeSandboxImagePrepullFailed = "21006"
	// CodeSandboxCapabilityUnavailable 表示运行时能力实现未注册或不可调用。
	CodeSandboxCapabilityUnavailable = "21007"
	// CodeSandboxImageDisableFailed 表示运行时镜像停用失败。
	CodeSandboxImageDisableFailed = "21008"
	// CodeSandboxSelftestRecycleConfigInvalid 表示运行时自检清理配置非法。
	CodeSandboxSelftestRecycleConfigInvalid = "21009"
)

const (
	// CodeSandboxNotFound 表示沙箱不存在或已释放。
	CodeSandboxNotFound = "22001"
	// CodeSandboxCreateFailed 表示沙箱创建失败。
	CodeSandboxCreateFailed = "22002"
	// CodeSandboxRecycleFailed 表示沙箱回收或销毁失败。
	CodeSandboxRecycleFailed = "22003"
	// CodeSandboxStateInvalid 表示当前状态不允许该操作。
	CodeSandboxStateInvalid = "22004"
	// CodeSandboxTimeout 表示启动、等待或执行超时。
	CodeSandboxTimeout = "22005"
	// CodeSandboxFileInvalid 表示文件路径或内容非法。
	CodeSandboxFileInvalid = "22006"
	// CodeSandboxFileNotFound 表示文件不存在或不可读取。
	CodeSandboxFileNotFound = "22007"
	// CodeSandboxFilePersistFailed 表示文件持久化失败。
	CodeSandboxFilePersistFailed = "22008"
	// CodeSandboxInitFailed 表示初始化脚本或代码恢复失败。
	CodeSandboxInitFailed = "22009"
	// CodeSandboxChainFailed 表示链上能力调用失败。
	CodeSandboxChainFailed = "22010"
	// CodeSandboxExecFailed 表示内部判题命令执行失败。
	CodeSandboxExecFailed = "22011"
	// CodeSandboxContractRequestInvalid 表示内部契约层沙箱请求校验失败。
	CodeSandboxContractRequestInvalid = "22012"
	// CodeSandboxRuntimeCreateInvalid 表示运行时注册请求非法。
	CodeSandboxRuntimeCreateInvalid = "22013"
	// CodeSandboxRuntimeUpdateInvalid 表示运行时更新请求非法。
	CodeSandboxRuntimeUpdateInvalid = "22014"
	// CodeSandboxImageCreateInvalid 表示运行时镜像登记请求非法。
	CodeSandboxImageCreateInvalid = "22015"
	// CodeSandboxImagePrepullParamInvalid 表示运行时镜像预拉取路径参数非法。
	CodeSandboxImagePrepullParamInvalid = "22016"
	// CodeSandboxCreateRequestInvalid 表示沙箱创建请求非法。
	CodeSandboxCreateRequestInvalid = "22017"
	// CodeSandboxOwnerInvalid 表示沙箱使用者信息非法。
	CodeSandboxOwnerInvalid = "22018"
	// CodeSandboxRecycleRequestInvalid 表示来源级联回收请求非法。
	CodeSandboxRecycleRequestInvalid = "22019"
	// CodeSandboxDeployRequestInvalid 表示合约部署请求非法。
	CodeSandboxDeployRequestInvalid = "22020"
	// CodeSandboxTxRequestInvalid 表示链上交易请求非法。
	CodeSandboxTxRequestInvalid = "22021"
	// CodeSandboxFileWriteRequestInvalid 表示文件写入请求非法。
	CodeSandboxFileWriteRequestInvalid = "22022"
	// CodeSandboxOwnershipInvalid 表示沙箱归属校验失败。
	CodeSandboxOwnershipInvalid = "22023"
	// CodeSandboxStatePersistFailed 表示沙箱状态持久化失败。
	CodeSandboxStatePersistFailed = "22024"
	// CodeSandboxAuditFailed 表示沙箱审计写入失败。
	CodeSandboxAuditFailed = "22025"
	// CodeSandboxSnapshotUnavailable 表示集群未安装或未启用 CSI 快照能力。
	CodeSandboxSnapshotUnavailable = "22026"
	// CodeSandboxRecycleConfigInvalid 表示回收调度器配置非法。
	CodeSandboxRecycleConfigInvalid = "22027"
	// CodeSandboxRecycleScanFailed 表示回收调度器扫描候选沙箱失败。
	CodeSandboxRecycleScanFailed = "22028"
	// CodeSandboxRecycleItemFailed 表示回收调度器处理单个沙箱失败。
	CodeSandboxRecycleItemFailed = "22029"
	// CodeSandboxSnapshotCleanupFailed 表示快照保留命名空间到期清理失败。
	CodeSandboxSnapshotCleanupFailed = "22030"
	// CodeSandboxResourceUsageFailed 表示沙箱资源用量查询失败。
	CodeSandboxResourceUsageFailed = "22031"
	// CodeSandboxImageDisableParamInvalid 表示运行时镜像停用路径参数非法。
	CodeSandboxImageDisableParamInvalid = "22032"
	// CodeSandboxPrivateArchiveInvalid 表示私有判题归档注入请求非法。
	CodeSandboxPrivateArchiveInvalid = "22033"
)

const (
	// CodeSandboxToolNotFound 表示工具不存在或停用。
	CodeSandboxToolNotFound = "23001"
	// CodeSandboxToolIncompatible 表示工具与运行时不兼容。
	CodeSandboxToolIncompatible = "23002"
	// CodeSandboxToolProxyUnavailable 表示工具代理不可达。
	CodeSandboxToolProxyUnavailable = "23003"
	// CodeSandboxToolCreateInvalid 表示工具注册请求非法。
	CodeSandboxToolCreateInvalid = "23004"
	// CodeSandboxToolPersistFailed 表示工具配置持久化失败。
	CodeSandboxToolPersistFailed = "23005"
)

const (
	// CodeSandboxQuotaExceeded 表示沙箱并发数量超过配额。
	CodeSandboxQuotaExceeded = "24001"
	// CodeSandboxQuotaInvalid 表示配额配置非法。
	CodeSandboxQuotaInvalid = "24002"
	// CodeSandboxClusterBusy 表示集群资源或排队容量不足。
	CodeSandboxClusterBusy = "24003"
	// CodeSandboxQuotaUpdateInvalid 表示配额调整请求非法。
	CodeSandboxQuotaUpdateInvalid = "24004"
	// CodeSandboxQuotaPersistFailed 表示配额持久化失败。
	CodeSandboxQuotaPersistFailed = "24005"
	// CodeSandboxKeepaliveQuotaExceeded 表示保活时长或保活能力超过配额。
	CodeSandboxKeepaliveQuotaExceeded = "24006"
	// CodeSandboxSnapshotQuotaExceeded 表示快照保留时长或快照能力超过配额。
	CodeSandboxSnapshotQuotaExceeded = "24007"
	// CodeSandboxResourceQuotaExceeded 表示租户 CPU 或内存总容量超过配额。
	CodeSandboxResourceQuotaExceeded = "24008"
)

var (
	// ErrSandboxRuntimeNotFound 表示运行环境不存在。
	ErrSandboxRuntimeNotFound = New(CodeSandboxRuntimeNotFound, "运行环境不存在")
	// ErrSandboxRuntimeUnavailable 表示运行环境暂不可用。
	ErrSandboxRuntimeUnavailable = New(CodeSandboxRuntimeUnavailable, "运行环境暂不可用")
	// ErrSandboxRuntimeConfigInvalid 表示运行环境配置不正确。
	ErrSandboxRuntimeConfigInvalid = New(CodeSandboxRuntimeConfigInvalid, "运行环境配置不正确")
	// ErrSandboxRuntimeImageNotFound 表示运行环境镜像不存在。
	ErrSandboxRuntimeImageNotFound = New(CodeSandboxRuntimeImageNotFound, "运行环境镜像不存在")
	// ErrSandboxSelftestFailed 表示运行环境自检未通过。
	ErrSandboxSelftestFailed = New(CodeSandboxSelftestFailed, "运行环境自检未通过")
	// ErrSandboxImagePrepullFailed 表示运行环境镜像准备失败。
	ErrSandboxImagePrepullFailed = New(CodeSandboxImagePrepullFailed, "运行环境镜像准备失败,请稍后重试")
	// ErrSandboxCapabilityUnavailable 表示运行环境能力暂不可用。
	ErrSandboxCapabilityUnavailable = New(CodeSandboxCapabilityUnavailable, "运行环境能力暂不可用")
	// ErrSandboxImageDisableFailed 表示运行环境镜像停用失败。
	ErrSandboxImageDisableFailed = New(CodeSandboxImageDisableFailed, "运行环境镜像停用失败,请稍后重试")
	// ErrSandboxSelftestRecycleConfigInvalid 表示自检清理配置不正确。
	ErrSandboxSelftestRecycleConfigInvalid = New(CodeSandboxSelftestRecycleConfigInvalid, "运行环境自检配置不正确")
)

var (
	// ErrSandboxNotFound 表示实验环境不存在或已释放。
	ErrSandboxNotFound = New(CodeSandboxNotFound, "实验环境不存在或已释放")
	// ErrSandboxCreateFailed 表示实验环境创建失败。
	ErrSandboxCreateFailed = New(CodeSandboxCreateFailed, "实验环境创建失败,请稍后重试")
	// ErrSandboxRecycleFailed 表示实验环境释放失败。
	ErrSandboxRecycleFailed = New(CodeSandboxRecycleFailed, "实验环境释放失败,请稍后重试")
	// ErrSandboxStateInvalid 表示当前状态不支持该操作。
	ErrSandboxStateInvalid = New(CodeSandboxStateInvalid, "实验环境当前状态不支持该操作")
	// ErrSandboxTimeout 表示实验环境响应超时。
	ErrSandboxTimeout = New(CodeSandboxTimeout, "实验环境响应超时,请稍后重试")
	// ErrSandboxFileInvalid 表示文件路径或内容不正确。
	ErrSandboxFileInvalid = New(CodeSandboxFileInvalid, "文件路径或内容不正确")
	// ErrSandboxFileNotFound 表示文件不存在或暂时无法读取。
	ErrSandboxFileNotFound = New(CodeSandboxFileNotFound, "文件不存在或暂时无法读取")
	// ErrSandboxFilePersistFailed 表示文件保存失败。
	ErrSandboxFilePersistFailed = New(CodeSandboxFilePersistFailed, "文件保存失败,请稍后重试")
	// ErrSandboxInitFailed 表示实验环境初始化失败。
	ErrSandboxInitFailed = New(CodeSandboxInitFailed, "实验环境初始化失败,请稍后重试")
	// ErrSandboxChainFailed 表示链上操作失败。
	ErrSandboxChainFailed = New(CodeSandboxChainFailed, "链上操作失败,请稍后重试")
	// ErrSandboxExecFailed 表示实验环境执行失败。
	ErrSandboxExecFailed = New(CodeSandboxExecFailed, "实验环境执行失败,请稍后重试")
	// ErrSandboxContractRequestInvalid 表示内部沙箱请求信息不完整。
	ErrSandboxContractRequestInvalid = New(CodeSandboxContractRequestInvalid, "实验环境请求信息不完整,请检查后重试")
	// ErrSandboxRuntimeCreateInvalid 表示运行环境注册信息不完整。
	ErrSandboxRuntimeCreateInvalid = New(CodeSandboxRuntimeCreateInvalid, "运行环境注册信息不完整,请检查后重试")
	// ErrSandboxRuntimeUpdateInvalid 表示运行环境更新信息不完整。
	ErrSandboxRuntimeUpdateInvalid = New(CodeSandboxRuntimeUpdateInvalid, "运行环境更新信息不完整,请检查后重试")
	// ErrSandboxImageCreateInvalid 表示运行环境镜像信息不完整。
	ErrSandboxImageCreateInvalid = New(CodeSandboxImageCreateInvalid, "运行环境镜像信息不完整,请检查后重试")
	// ErrSandboxImagePrepullParamInvalid 表示镜像预拉取参数不正确。
	ErrSandboxImagePrepullParamInvalid = New(CodeSandboxImagePrepullParamInvalid, "运行环境镜像预拉取参数不正确,请检查后重试")
	// ErrSandboxCreateRequestInvalid 表示实验环境创建信息不完整。
	ErrSandboxCreateRequestInvalid = New(CodeSandboxCreateRequestInvalid, "实验环境创建信息不完整,请检查后重试")
	// ErrSandboxOwnerInvalid 表示实验环境使用者信息不正确。
	ErrSandboxOwnerInvalid = New(CodeSandboxOwnerInvalid, "实验环境使用者信息不正确,请检查后重试")
	// ErrSandboxRecycleRequestInvalid 表示实验环境回收信息不完整。
	ErrSandboxRecycleRequestInvalid = New(CodeSandboxRecycleRequestInvalid, "实验环境回收信息不完整,请检查后重试")
	// ErrSandboxDeployRequestInvalid 表示合约部署请求信息不完整。
	ErrSandboxDeployRequestInvalid = New(CodeSandboxDeployRequestInvalid, "合约部署请求信息不完整,请检查后重试")
	// ErrSandboxTxRequestInvalid 表示链上交易请求信息不完整。
	ErrSandboxTxRequestInvalid = New(CodeSandboxTxRequestInvalid, "链上交易请求信息不完整,请检查后重试")
	// ErrSandboxFileWriteRequestInvalid 表示文件写入信息不完整。
	ErrSandboxFileWriteRequestInvalid = New(CodeSandboxFileWriteRequestInvalid, "文件写入信息不完整,请检查后重试")
	// ErrSandboxOwnershipInvalid 表示无法访问该实验环境。
	ErrSandboxOwnershipInvalid = New(CodeSandboxOwnershipInvalid, "无法访问该实验环境")
	// ErrSandboxStatePersistFailed 表示实验环境状态保存失败。
	ErrSandboxStatePersistFailed = New(CodeSandboxStatePersistFailed, "实验环境状态保存失败,请稍后重试")
	// ErrSandboxAuditFailed 表示操作记录保存失败。
	ErrSandboxAuditFailed = New(CodeSandboxAuditFailed, "操作记录保存失败,请稍后重试")
	// ErrSandboxSnapshotUnavailable 表示实验环境快照能力暂不可用。
	ErrSandboxSnapshotUnavailable = New(CodeSandboxSnapshotUnavailable, "实验环境快照能力暂不可用")
	// ErrSandboxRecycleConfigInvalid 表示实验环境回收配置不正确。
	ErrSandboxRecycleConfigInvalid = New(CodeSandboxRecycleConfigInvalid, "实验环境回收配置不正确")
	// ErrSandboxRecycleScanFailed 表示实验环境回收任务扫描失败。
	ErrSandboxRecycleScanFailed = New(CodeSandboxRecycleScanFailed, "实验环境回收任务扫描失败,请稍后重试")
	// ErrSandboxRecycleItemFailed 表示实验环境回收任务处理失败。
	ErrSandboxRecycleItemFailed = New(CodeSandboxRecycleItemFailed, "实验环境回收任务处理失败,请稍后重试")
	// ErrSandboxSnapshotCleanupFailed 表示实验环境快照清理失败。
	ErrSandboxSnapshotCleanupFailed = New(CodeSandboxSnapshotCleanupFailed, "实验环境快照清理失败,请稍后重试")
	// ErrSandboxResourceUsageFailed 表示实验环境资源信息暂时无法读取。
	ErrSandboxResourceUsageFailed = New(CodeSandboxResourceUsageFailed, "实验环境资源信息暂时无法读取,请稍后重试")
	// ErrSandboxImageDisableParamInvalid 表示运行环境镜像停用参数不正确。
	ErrSandboxImageDisableParamInvalid = New(CodeSandboxImageDisableParamInvalid, "运行环境镜像停用参数不正确,请检查后重试")
	// ErrSandboxPrivateArchiveInvalid 表示判题私有输入准备失败。
	ErrSandboxPrivateArchiveInvalid = New(CodeSandboxPrivateArchiveInvalid, "判题输入准备失败,请稍后重试")
)

var (
	// ErrSandboxToolNotFound 表示工具不存在或已停用。
	ErrSandboxToolNotFound = New(CodeSandboxToolNotFound, "工具不存在或已停用")
	// ErrSandboxToolIncompatible 表示所选工具不适用于该运行环境。
	ErrSandboxToolIncompatible = New(CodeSandboxToolIncompatible, "所选工具不适用于该运行环境")
	// ErrSandboxToolProxyUnavailable 表示工具暂时无法打开。
	ErrSandboxToolProxyUnavailable = New(CodeSandboxToolProxyUnavailable, "工具暂时无法打开,请稍后重试")
	// ErrSandboxToolCreateInvalid 表示工具注册信息不完整。
	ErrSandboxToolCreateInvalid = New(CodeSandboxToolCreateInvalid, "工具注册信息不完整,请检查后重试")
	// ErrSandboxToolPersistFailed 表示工具配置保存失败。
	ErrSandboxToolPersistFailed = New(CodeSandboxToolPersistFailed, "工具配置保存失败,请稍后重试")
)

var (
	// ErrSandboxQuotaExceeded 表示学校实验环境数量已达上限。
	ErrSandboxQuotaExceeded = New(CodeSandboxQuotaExceeded, "当前学校实验环境数量已达上限,请稍后再试")
	// ErrSandboxQuotaInvalid 表示资源配额配置不正确。
	ErrSandboxQuotaInvalid = New(CodeSandboxQuotaInvalid, "资源配额配置不正确")
	// ErrSandboxClusterBusy 表示实验环境资源繁忙。
	ErrSandboxClusterBusy = New(CodeSandboxClusterBusy, "实验环境资源繁忙,请稍后再试")
	// ErrSandboxQuotaUpdateInvalid 表示资源配额调整信息不完整。
	ErrSandboxQuotaUpdateInvalid = New(CodeSandboxQuotaUpdateInvalid, "资源配额调整信息不完整,请检查后重试")
	// ErrSandboxQuotaPersistFailed 表示资源配额保存失败。
	ErrSandboxQuotaPersistFailed = New(CodeSandboxQuotaPersistFailed, "资源配额保存失败,请稍后重试")
	// ErrSandboxKeepaliveQuotaExceeded 表示保活配置超过学校允许范围。
	ErrSandboxKeepaliveQuotaExceeded = New(CodeSandboxKeepaliveQuotaExceeded, "环境保活时长超过学校允许范围,请调整后重试")
	// ErrSandboxSnapshotQuotaExceeded 表示快照配置超过学校允许范围。
	ErrSandboxSnapshotQuotaExceeded = New(CodeSandboxSnapshotQuotaExceeded, "环境快照保留时长超过学校允许范围,请调整后重试")
	// ErrSandboxResourceQuotaExceeded 表示学校 CPU 或内存资源容量不足。
	ErrSandboxResourceQuotaExceeded = New(CodeSandboxResourceQuotaExceeded, "学校实验环境资源容量不足,请稍后再试")
)
