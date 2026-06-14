// content dto 文件定义 M5 HTTP 请求与响应结构。
package content

// ItemDTO 是内容外壳响应。
type ItemDTO struct {
	ID              int64    `json:"id"`
	TenantID        int64    `json:"tenant_id"`
	Code            string   `json:"code"`
	Version         string   `json:"version"`
	Type            int16    `json:"type"`
	Title           string   `json:"title"`
	CategoryID      int64    `json:"category_id,omitempty"`
	Difficulty      int16    `json:"difficulty"`
	Tags            []string `json:"tags"`
	KnowledgePoints []string `json:"knowledge_points"`
	AuthorID        int64    `json:"author_id"`
	AuthorType      int16    `json:"author_type"`
	Visibility      int16    `json:"visibility"`
	Status          int16    `json:"status"`
	UsageCount      int32    `json:"usage_count"`
	VersionHash     string   `json:"version_hash"`
	CreatedAt       string   `json:"created_at"`
	UpdatedAt       string   `json:"updated_at"`
}

// ItemSnapshotDTO 是带题面或全量正文的响应。
type ItemSnapshotDTO struct {
	ItemDTO
	Body            map[string]any `json:"body"`
	SensitiveFields []string       `json:"sensitive_fields,omitempty"`
}

// CreateItemRequest 是教师创建草稿内容请求。
type CreateItemRequest struct {
	Code            string         `json:"code"`
	Version         string         `json:"version"`
	Type            int16          `json:"type"`
	Title           string         `json:"title"`
	CategoryID      int64          `json:"category_id"`
	Difficulty      int16          `json:"difficulty"`
	Tags            []string       `json:"tags"`
	KnowledgePoints []string       `json:"knowledge_points"`
	Visibility      int16          `json:"visibility"`
	Body            map[string]any `json:"body"`
	SensitiveFields []string       `json:"sensitive_fields"`
}

// UpdateItemRequest 是草稿编辑请求。
type UpdateItemRequest struct {
	Title           string         `json:"title"`
	CategoryID      int64          `json:"category_id"`
	Difficulty      int16          `json:"difficulty"`
	Tags            []string       `json:"tags"`
	KnowledgePoints []string       `json:"knowledge_points"`
	Visibility      int16          `json:"visibility"`
	Body            map[string]any `json:"body"`
	SensitiveFields []string       `json:"sensitive_fields"`
}

// NewVersionRequest 是从既有版本复制出新草稿的请求。
type NewVersionRequest struct {
	SourceVersion string `json:"source_version"`
	NewVersion    string `json:"new_version"`
}

// CloneItemRequest 是克隆共享或本租户内容的请求。
type CloneItemRequest struct {
	NewCode    string `json:"new_code"`
	NewVersion string `json:"new_version"`
}

// AttachmentDownloadGrantRequest 是附件短时下载授权请求。
type AttachmentDownloadGrantRequest struct {
	ResourceID string `json:"resource_id"`
	ObjectRef  string `json:"object_ref"`
}

// BatchItemsRequest 是内部批量题面读取请求。
type BatchItemsRequest struct {
	Items []ItemRefDTO `json:"items"`
}

// ItemRefDTO 是 HTTP 层内容引用。
type ItemRefDTO struct {
	Code    string `json:"code"`
	Version string `json:"version"`
}

// SystemImportRequest 是内部系统建题 HTTP 请求。
type SystemImportRequest struct {
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

// CategoryRequest 是分类创建和更新请求。
type CategoryRequest struct {
	ParentID int64  `json:"parent_id"`
	Name     string `json:"name"`
	Sort     int32  `json:"sort"`
}

// CategoryDTO 是分类响应。
type CategoryDTO struct {
	ID        int64  `json:"id"`
	ParentID  int64  `json:"parent_id,omitempty"`
	Name      string `json:"name"`
	Sort      int32  `json:"sort"`
	CreatedAt string `json:"created_at"`
	UpdatedAt string `json:"updated_at"`
}

// CreatePaperRequest 是组卷请求。
type CreatePaperRequest struct {
	Name        string              `json:"name"`
	GenMode     int16               `json:"gen_mode"`
	GenCriteria PaperCriteria       `json:"gen_criteria"`
	Items       []PaperItemInputDTO `json:"items"`
}

// PaperItemInputDTO 是手动组卷题目输入。
type PaperItemInputDTO struct {
	Code    string `json:"code"`
	Version string `json:"version"`
	Score   int32  `json:"score"`
}

// PaperDTO 是组卷元信息响应。
type PaperDTO struct {
	ID          int64         `json:"id"`
	Name        string        `json:"name"`
	AuthorID    int64         `json:"author_id"`
	GenMode     int16         `json:"gen_mode"`
	GenCriteria PaperCriteria `json:"gen_criteria"`
	CreatedAt   string        `json:"created_at"`
	UpdatedAt   string        `json:"updated_at"`
}

// PaperDetailDTO 是试卷详情响应。
type PaperDetailDTO struct {
	Paper PaperDTO           `json:"paper"`
	Items []PaperItemFaceDTO `json:"items"`
}

// PaperItemFaceDTO 是试卷题目题面响应。
type PaperItemFaceDTO struct {
	ID      int64          `json:"id"`
	Code    string         `json:"code"`
	Version string         `json:"version"`
	Score   int32          `json:"score"`
	Seq     int32          `json:"seq"`
	Item    ItemDTO        `json:"item"`
	Body    map[string]any `json:"body"`
}
