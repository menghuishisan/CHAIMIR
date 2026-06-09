// M7 转换层:处理领域 DTO、contracts DTO 与 HTTP 输出结构之间的纯转换。
package experiment

import (
	"chaimir/internal/platform/jsonx"
	"chaimir/pkg/apperr"
)

// componentsBytes 序列化组件编排 JSONB。
func componentsBytes(v ExperimentComponents) ([]byte, error) {
	return jsonx.AnyBytes(v, apperr.ErrExperimentInvalid)
}

// componentsValue 解析组件编排 JSONB。
func componentsValue(data []byte) ExperimentComponents {
	return jsonx.Decode(data, ExperimentComponents{})
}

// sandboxRefsBytes 序列化沙箱引用列表。
func sandboxRefsBytes(refs []SandboxRef) ([]byte, error) {
	if refs == nil {
		refs = []SandboxRef{}
	}
	return jsonx.AnyBytes(refs, apperr.ErrExperimentInstanceInvalid)
}

// sandboxRefsValue 解析沙箱引用列表。
func sandboxRefsValue(data []byte) []SandboxRef {
	return jsonx.Decode(data, []SandboxRef{})
}

// simRefsBytes 序列化仿真会话引用列表。
func simRefsBytes(refs []SimSessionRef) ([]byte, error) {
	if refs == nil {
		refs = []SimSessionRef{}
	}
	return jsonx.AnyBytes(refs, apperr.ErrExperimentInstanceInvalid)
}

// simRefsValue 解析仿真会话引用列表。
func simRefsValue(data []byte) []SimSessionRef {
	return jsonx.Decode(data, []SimSessionRef{})
}
