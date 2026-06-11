// content model 文件定义 M5 service 与 repo 之间传递的领域模型。
package content

import "time"

// Item 表示内容外壳元信息。
type Item struct {
	ID              int64
	TenantID        int64
	Code            string
	Version         string
	Type            int16
	Title           string
	CategoryID      int64
	Difficulty      int16
	Tags            []string
	KnowledgePoints []string
	AuthorID        int64
	AuthorType      int16
	Visibility      int16
	Status          int16
	UsageCount      int32
	VersionHash     string
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

// ItemWithBody 表示内容外壳与类型化正文的完整快照。
type ItemWithBody struct {
	Item
	Body            map[string]any
	SensitiveFields []string
}

// Category 表示内容分类树节点。
type Category struct {
	ID        int64
	TenantID  int64
	ParentID  int64
	Name      string
	Sort      int32
	CreatedAt time.Time
	UpdatedAt time.Time
}

// Paper 表示组卷元信息。
type Paper struct {
	ID          int64
	TenantID    int64
	Name        string
	AuthorID    int64
	GenMode     int16
	GenCriteria PaperCriteria
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

// PaperItem 表示试卷内锁定版本的题目引用。
type PaperItem struct {
	ID          int64
	TenantID    int64
	PaperID     int64
	ItemCode    string
	ItemVersion string
	Score       int32
	Seq         int32
	CreatedAt   time.Time
}

// PaperCriteria 表示随机组卷条件。
type PaperCriteria struct {
	Type            int16    `json:"type,omitempty"`
	Difficulties    []int16  `json:"difficulty,omitempty"`
	KnowledgePoints []string `json:"knowledge_points,omitempty"`
	Count           int32    `json:"count,omitempty"`
	DefaultScore    int32    `json:"default_score,omitempty"`
}

// PaperWithItems 表示试卷详情及题面快照。
type PaperWithItems struct {
	Paper Paper
	Items []PaperItemFace
}

// PaperItemFace 表示试卷题目引用和对应题面。
type PaperItemFace struct {
	PaperItem
	Item ItemDTO
	Body map[string]any
}

// ItemListFilter 表示内容检索条件。
type ItemListFilter struct {
	Type            int16
	CategoryID      int64
	Difficulty      int16
	Tag             string
	KnowledgePoint  string
	Keyword         string
	Visibility      int16
	Status          int16
	AuthorID        int64
	OnlyShared      bool
	PublishedShared bool
	Page            int
	Size            int
}
