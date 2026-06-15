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
type ContentReadService interface {
	// GetContentFace 按锁定版本读取题面视角内容,敏感字段已被剥离。
	GetContentFace(ctx context.Context, tenantID int64, ref ContentItemRef) (ContentItemSnapshot, error)
	// GetContentFull 按锁定版本读取全量内容,仅供内部服务或受控教师路径使用。
	GetContentFull(ctx context.Context, tenantID int64, ref ContentItemRef) (ContentItemSnapshot, error)
	// BatchGetContentFace 批量读取题面内容,供组卷展开或题目列表批量渲染使用。
	BatchGetContentFace(ctx context.Context, tenantID int64, refs []ContentItemRef) ([]ContentItemSnapshot, error)
	// IncrementUsage 记录内容被业务引用,用于删除保护与复用统计。
	IncrementUsage(ctx context.Context, tenantID int64, ref ContentItemRef) error
}

// ContentJudgeReadService 是 M5 对 M3 判题路径暴露的只读判题配置契约。
type ContentJudgeReadService interface {
	// GetJudgeSpec 按租户与锁定版本读取判题配置与答案快照,强制保留多租户边界。
	GetJudgeSpec(ctx context.Context, tenantID int64, itemCode, itemVersion string) (ContentJudgeSpec, error)
}

// ContentImportService 是 M5 对 M8 等内部模块暴露的系统建题契约。
type ContentImportService interface {
	// SystemImportContent 把预验证后的自包含题目固化到内容中心。
	SystemImportContent(ctx context.Context, req ContentSystemImportRequest) (ContentItemSnapshot, error)
}
