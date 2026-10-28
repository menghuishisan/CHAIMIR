// dto_composition 定义编排连接预览的最小输入和安全输出,不包含执行规格或镜像地址。
package sandbox

import "chaimir/internal/contracts"

// CompositionPreviewRequest 只接收场景无关的选择和连接,不替业务决定访问边界。
type CompositionPreviewRequest struct {
	Runtimes                 []contracts.CompositionRuntimeRef   `json:"runtimes"`
	WorkspaceRuntimeInstance string                              `json:"workspace_runtime_instance"`
	Infra                    []contracts.CompositionComponentRef `json:"infra"`
	Tools                    []contracts.CompositionComponentRef `json:"tools"`
	Links                    []contracts.CompositionLink         `json:"links"`
}

// CompositionPreviewResponse 返回本次依赖、连接候选与完整性,不发布快照。
type CompositionPreviewResponse struct {
	Infra    []contracts.CompositionComponentRef `json:"infra"`
	Bindings []CompositionBindingPreview         `json:"bindings"`
	Links    []contracts.CompositionLink         `json:"links"`
	Complete bool                                `json:"complete"`
}

// CompositionBindingPreview 描述一个组件连接要求,内部绑定仅作稳定识别。
type CompositionBindingPreview struct {
	SourceComponent string                     `json:"source_component"`
	SourceName      string                     `json:"source_name"`
	ConfigBinding   string                     `json:"config_binding"`
	Options         []CompositionBindingOption `json:"options"`
}

// CompositionBindingOption 的连接由M2目录和端点解析生成,浏览器只能选择目标。
type CompositionBindingOption struct {
	Label string                    `json:"label"`
	Link  contracts.CompositionLink `json:"link"`
}
