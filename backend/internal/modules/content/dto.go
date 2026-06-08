// M5 DTO 定义:承载内容、分类、组卷与内部取用的请求响应结构。
package content

import "time"

// CreateItemRequest 是创建内容草稿的请求。
type CreateItemRequest struct {
	Code             string         `json:"code"`
	Version          string         `json:"version"`
	Type             int16          `json:"type"`
	Title            string         `json:"title"`
	CategoryID       string         `json:"category_id"`
	Difficulty       int16          `json:"difficulty"`
	Tags             []string       `json:"tags"`
	KnowledgePoints  []string       `json:"knowledge_points"`
	AuthorID         string         `json:"author_id"`
	AuthorType       int16          `json:"author_type"`
	Visibility       int16          `json:"visibility"`
	Body             map[string]any `json:"body"`
	SensitiveFields  []string       `json:"sensitive_fields"`
	AutoPublish      bool           `json:"auto_publish"`
	SystemImportNote map[string]any `json:"system_import_note"`
}

// UpdateItemRequest 是编辑草稿内容的请求。
type UpdateItemRequest struct {
	Title           string         `json:"title"`
	CategoryID      string         `json:"category_id"`
	Difficulty      int16          `json:"difficulty"`
	Tags            []string       `json:"tags"`
	KnowledgePoints []string       `json:"knowledge_points"`
	Visibility      int16          `json:"visibility"`
	Body            map[string]any `json:"body"`
	SensitiveFields []string       `json:"sensitive_fields"`
}

// ItemDTO 是内容摘要或内容详情响应。
type ItemDTO struct {
	ID              string         `json:"id,omitempty"`
	TenantID        string         `json:"tenant_id,omitempty"`
	Code            string         `json:"code"`
	Version         string         `json:"version"`
	Type            int16          `json:"type"`
	Title           string         `json:"title"`
	CategoryID      string         `json:"category_id,omitempty"`
	Difficulty      int16          `json:"difficulty"`
	Tags            []string       `json:"tags"`
	KnowledgePoints []string       `json:"knowledge_points"`
	AuthorID        string         `json:"author_id"`
	AuthorType      int16          `json:"author_type"`
	Visibility      int16          `json:"visibility"`
	Status          int16          `json:"status"`
	UsageCount      int32          `json:"usage_count"`
	BodyHash        string         `json:"body_hash,omitempty"`
	Body            map[string]any `json:"body,omitempty"`
	SensitiveFields []string       `json:"sensitive_fields,omitempty"`
	CreatedAt       time.Time      `json:"created_at,omitempty"`
	UpdatedAt       time.Time      `json:"updated_at,omitempty"`
}

// ListItemsRequest 是内容检索条件。
type ListItemsRequest struct {
	Type       int16
	CategoryID string
	Difficulty int16
	Tag        string
	KP         string
	Keyword    string
	Visibility int16
	Status     int16
	Page       int
	Size       int
}

// CloneRequest 是克隆内容请求。
type CloneRequest struct {
	Code string `json:"code"`
}

// NewVersionRequest 是发新版请求,显式区分源版本和目标新版本以避免版本语义混用。
type NewVersionRequest struct {
	SourceVersion string `json:"source_version"`
	Version       string `json:"version"`
}

// BatchGetRequest 是批量取题面请求。
type BatchGetRequest struct {
	Items []ItemRef `json:"items"`
}

// ItemRef 是锁定版本内容引用。
type ItemRef struct {
	Code    string `json:"code"`
	Version string `json:"version"`
}

// CategoryRequest 是分类维护请求。
type CategoryRequest struct {
	ParentID string `json:"parent_id"`
	Name     string `json:"name"`
	Sort     int32  `json:"sort"`
}

// CategoryDTO 是分类树节点摘要。
type CategoryDTO struct {
	ID        string    `json:"id"`
	ParentID  string    `json:"parent_id,omitempty"`
	Name      string    `json:"name"`
	Sort      int32     `json:"sort"`
	CreatedAt time.Time `json:"created_at,omitempty"`
}

// PaperRequest 是创建或重新生成试卷的请求。
type PaperRequest struct {
	Name        string         `json:"name"`
	GenMode     int16          `json:"gen_mode"`
	GenCriteria map[string]any `json:"gen_criteria"`
	Items       []PaperItemReq `json:"items"`
}

// PaperItemReq 是手动组卷题目请求。
type PaperItemReq struct {
	Code    string `json:"code"`
	Version string `json:"version"`
	Score   int32  `json:"score"`
	Seq     int32  `json:"seq"`
}

// PaperDTO 是试卷详情响应。
type PaperDTO struct {
	ID          string         `json:"id"`
	Name        string         `json:"name"`
	AuthorID    string         `json:"author_id"`
	GenMode     int16          `json:"gen_mode"`
	GenCriteria map[string]any `json:"gen_criteria"`
	Items       []PaperItemDTO `json:"items,omitempty"`
	CreatedAt   time.Time      `json:"created_at,omitempty"`
}

// PaperItemDTO 是试卷中的锁定版本题目。
type PaperItemDTO struct {
	ID      string  `json:"id,omitempty"`
	Code    string  `json:"code"`
	Version string  `json:"version"`
	Score   int32   `json:"score"`
	Seq     int32   `json:"seq"`
	Item    ItemDTO `json:"item,omitempty"`
}
