// contracts 定义第 1 层题库与模板中心对外暴露的内容读取、判题配置与系统导入契约。
package contracts

import "context"

// ContentItemRef 是业务模块锁定内容版本时使用的稳定引用。
type ContentItemRef struct {
	ItemCode    string `json:"item_code"`
	ItemVersion string `json:"item_version"`
}

// ContentItemSnapshot 是 M5 对外输出的题面或全量内容快照。
type ContentItemSnapshot struct {
	ItemCode        string         `json:"item_code"`
	ItemVersion     string         `json:"item_version"`
	Type            int16          `json:"type"`
	Title           string         `json:"title"`
	Difficulty      int16          `json:"difficulty"`
	Visibility      int16          `json:"visibility"`
	Tags            []string       `json:"tags"`
	KnowledgePoints []string       `json:"knowledge_points"`
	Body            map[string]any `json:"body"`
	VersionHash     string         `json:"version_hash"`
	Status          int16          `json:"status"`
}

// ContentJudgeSpec 是 M5 提供给 M3 的黑盒判题配置快照。
type ContentJudgeSpec struct {
	ItemCode    string         `json:"item_code"`
	ItemVersion string         `json:"item_version"`
	JudgerCode  string         `json:"judger_code"`
	MaxScore    int32          `json:"max_score"`
	SuiteRef    string         `json:"suite_ref"`
	Expectation map[string]any `json:"expectation"`
	VersionHash string         `json:"version_hash"`
}

// ContentSystemImportRequest 是系统或外部源固化内容时的内部请求。
type ContentSystemImportRequest struct {
	TenantID         int64          `json:"tenant_id"`
	Code             string         `json:"code"`
	Version          string         `json:"version"`
	Type             int16          `json:"type"`
	Title            string         `json:"title"`
	CategoryID       int64          `json:"category_id"`
	Difficulty       int16          `json:"difficulty"`
	Tags             []string       `json:"tags"`
	KnowledgePoints  []string       `json:"knowledge_points"`
	AuthorID         int64          `json:"author_id"`
	AuthorType       int16          `json:"author_type"`
	Visibility       int16          `json:"visibility"`
	Body             map[string]any `json:"body"`
	SensitiveFields  []string       `json:"sensitive_fields"`
	AutoPublish      bool           `json:"auto_publish"`
	SystemImportNote map[string]any `json:"system_import_note"`
}

// ContentReadService 是 M5 对 M2/M4/M6/M7/M8 暴露的内容读取与引用计数契约。
//
// 硬约束:
// - 业务引用必须锁定 ContentItemRef 的 item_code + item_version,不得只传 item_code 或最新版本别名。
// - 普通展示、组卷展开和学生可见链路只能调用 GetContentFace/BatchGetContentFace,返回体必须已剥离 answer、flag、judge_config、testcases 等敏感字段。
// - GetContentFull 只能用于当前租户内的内部执行路径或 M5 受控教师作者路径,不得拿它跨租户读取共享库答案;跨校复用必须走 M5 clone 生成本租户独立草稿。
// - ReplaceUsageRefs 是内容引用计数唯一写入口,调用方必须传稳定 source_scope + source_ref,由 M5 幂等维护引用集合和 usage_count。
type ContentReadService interface {
	// GetContentFace 按锁定版本读取题面视角内容,敏感字段已被剥离。
	GetContentFace(ctx context.Context, tenantID int64, ref ContentItemRef) (ContentItemSnapshot, error)
	// GetContentFull 按锁定版本读取全量内容,仅供内部服务或受控教师路径使用。
	GetContentFull(ctx context.Context, tenantID int64, ref ContentItemRef) (ContentItemSnapshot, error)
	// BatchGetContentFace 批量读取题面内容,供组卷展开或题目列表批量渲染使用。
	BatchGetContentFace(ctx context.Context, tenantID int64, refs []ContentItemRef) ([]ContentItemSnapshot, error)
	// ReplaceUsageRefs 替换某业务来源持有的内容引用集合,用于删除保护与复用统计。
	ReplaceUsageRefs(ctx context.Context, tenantID int64, sourceScope, sourceRef string, refs []ContentItemRef) error
}

// ContentJudgeReadService 是 M5 对 M3 判题路径暴露的只读判题配置契约。
//
// 硬约束:该契约只供判题服务在隔离执行路径读取黑盒判题配置,不得把返回的 expectation、suite_ref 或 judge_config 派生内容回传给学生端。
type ContentJudgeReadService interface {
	// GetJudgeSpec 按租户与锁定版本读取判题配置与答案快照,强制保留多租户边界。
	GetJudgeSpec(ctx context.Context, tenantID int64, itemCode, itemVersion string) (ContentJudgeSpec, error)
}

// ContentImportService 是 M5 对 M8 等内部模块暴露的系统建题契约。
//
// 硬约束:调用方必须使用服务端已验签的租户上下文,请求体 TenantID 只能为空或与上下文一致;AuthorType 只能是系统或外部源,不得伪装教师来源。
type ContentImportService interface {
	// SystemImportContent 把预验证后的自包含题目固化到内容中心。
	SystemImportContent(ctx context.Context, req ContentSystemImportRequest) (ContentItemSnapshot, error)
}
