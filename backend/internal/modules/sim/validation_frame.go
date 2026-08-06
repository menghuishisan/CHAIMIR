// sim validation_frame 文件校验隔离容器回传的教学帧,是不可信输出进入前端前的最后一道闸门。
//
// 为什么后端必须自己有一份:容器执行的是外部提交的仿真包代码,它的输出可能给出越界模式数、
// 未登记的可视化模式、悬空 layout 引用或超规模状态。前端 `assertValidTeachingFrame` 有等价规则,
// 但只靠前端等于把协议边界交给不可信来源的下游 —— runner 一旦被绕过或替换,非法帧就直达浏览器。
// 校验规则与前端逐项对应(docs/04-仿真可视化引擎/03-可视化SDK与交互协议.md §1.1),
// 两侧同改不得分叉。
package sim

import (
	"fmt"

	"chaimir/internal/platform/jsonx"
	"chaimir/pkg/apperr"
)

// frameError 统一把协议问题包成 42011,技术细节只进日志。
func frameError(reason string) error {
	return apperr.ErrSimFrameInvalid.WithCause(fmt.Errorf("隔离执行返回的教学帧不合协议: %s", reason))
}

// validateBackendSnapshot 校验完整快照的必填段与教学帧协议。
func validateBackendSnapshot(snapshot BackendSnapshot, scaleLimit map[string]any) error {
	if snapshot.Tick < 0 {
		return frameError("tick 为负")
	}
	if len(snapshot.State) == 0 {
		return frameError("state 为空")
	}
	if err := validateBackendStateScale(snapshot.State, scaleLimit); err != nil {
		return frameError(err.Error())
	}
	return validateTeachingFrame(snapshot.View, scaleLimit)
}

// validateTeachingFrame 校验一帧的结构、职责声明、引用完整性与规模上限。
func validateTeachingFrame(view map[string]any, scaleLimit map[string]any) error {
	if len(view) == 0 {
		return frameError("view 为空")
	}
	if jsonx.StringField(view, "summary") == "" {
		return frameError("缺少 summary")
	}
	phase, ok := view["phase"].(map[string]any)
	if !ok || jsonx.StringField(phase, "id") == "" || jsonx.StringField(phase, "title") == "" {
		return frameError("缺少阶段声明")
	}
	focus, ok := view["focus"].(map[string]any)
	if !ok {
		return frameError("缺少观察焦点")
	}
	if primary, ok := focus["primary"].([]any); !ok || len(primary) == 0 {
		return frameError("观察焦点为空")
	}
	layout, ok := view["layout"].(map[string]any)
	if !ok || jsonx.StringField(layout, "primary") == "" {
		return frameError("缺少主视图声明")
	}
	patterns, ok := view["patterns"].([]any)
	if !ok || len(patterns) < 1 || len(patterns) > maxFramePatterns {
		return frameError("视图数量必须为 1 到 3 个")
	}

	patternIDs := make(map[string]struct{}, len(patterns))
	for _, item := range patterns {
		pattern, ok := item.(map[string]any)
		if !ok {
			return frameError("视图项结构无效")
		}
		id := jsonx.StringField(pattern, "id")
		if id == "" {
			return frameError("视图编号为空")
		}
		if _, exists := patternIDs[id]; exists {
			return frameError("视图编号重复")
		}
		if _, allowed := allowedPatternModes[jsonx.StringField(pattern, "mode")]; !allowed {
			return frameError("视图使用了未登记的可视化模式")
		}
		patternIDs[id] = struct{}{}
	}

	layoutIDs := layoutPatternIDs(layout)
	for id := range layoutIDs {
		if _, exists := patternIDs[id]; !exists {
			return frameError("layout 引用了不存在的视图")
		}
	}
	for id := range patternIDs {
		if _, used := layoutIDs[id]; !used {
			return frameError("存在未声明职责的视图")
		}
	}
	return nil
}

// layoutPatternIDs 汇总 layout 六个职责位引用到的视图编号。
func layoutPatternIDs(layout map[string]any) map[string]struct{} {
	ids := map[string]struct{}{}
	for _, key := range []string{"primary", "timeline", "trace"} {
		if id := jsonx.StringField(layout, key); id != "" {
			ids[id] = struct{}{}
		}
	}
	for _, key := range []string{"evidence", "metrics", "checkpoints"} {
		values, ok := layout[key].([]any)
		if !ok {
			continue
		}
		for _, value := range values {
			if id, ok := value.(string); ok && id != "" {
				ids[id] = struct{}{}
			}
		}
	}
	return ids
}

// validateBackendDescriptor 校验容器回传的包自描述信息。
//
// 它和快照一样是不可信输出,但危害点不同:描述里的 emits 决定页面会渲出哪些操作按钮,
// 若容器给出未登记的事件,学生点下去必然被服务端交互白名单拒绝 —— 那是一个必然失败的按钮。
// 故这里按已入库的 interaction_schema 逐项核对,并核对包身份,避免容器冒充另一个包。
func validateBackendDescriptor(descriptor BackendDescriptor, session SessionWithPackage) error {
	code := jsonx.StringField(descriptor.Meta, "code")
	version := jsonx.StringField(descriptor.Meta, "version")
	if code != session.PackageCode || version != session.PackageVersion {
		return frameError("包自描述信息的编号或版本与会话所引用的包不一致")
	}
	if len(descriptor.Interactions) == 0 {
		return frameError("包自描述信息没有声明任何操作")
	}
	if len(descriptor.Interactions) > maxDescriptorInteractions || len(descriptor.Narrative) > maxDescriptorNarrative || len(descriptor.Checkpoints) > maxDescriptorCheckpoints {
		return frameError("包自描述信息的条目数量超出允许范围")
	}
	for _, interaction := range descriptor.Interactions {
		if jsonx.StringField(interaction, "id") == "" {
			return frameError("操作声明缺少编号")
		}
		emits := jsonx.StringField(interaction, "emits")
		if _, ok := session.InteractionSchema.Events[emits]; !ok {
			return frameError("操作声明的事件未登记在包的交互白名单中: " + emits)
		}
	}
	for _, checkpoint := range descriptor.Checkpoints {
		if jsonx.StringField(checkpoint, "id") == "" || jsonx.StringField(checkpoint, "label") == "" {
			return frameError("检查点声明缺少编号或标题")
		}
	}
	for _, step := range descriptor.Narrative {
		if jsonx.StringField(step, "id") == "" {
			return frameError("教学步骤声明缺少编号")
		}
	}
	return nil
}
