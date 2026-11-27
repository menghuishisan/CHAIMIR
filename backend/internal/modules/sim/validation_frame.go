package sim

import (
	"encoding/json"
	"fmt"
	"slices"

	"chaimir/internal/platform/jsonx"
	"chaimir/pkg/apperr"
	"chaimir/pkg/jsonvalue"
)

func frameError(reason string) error {
	return apperr.ErrSimFrameInvalid.WithCause(fmt.Errorf("隔离执行返回的教学帧不合协议: %s", reason))
}

func validateBackendSnapshot(snapshot BackendSnapshot, scaleLimit map[string]any) error {
	value := map[string]any{
		"tick": snapshot.Tick, "state": snapshot.State, "view": snapshot.View,
		"interactionAvailability": snapshot.InteractionAvailability, "checkpointResults": snapshot.CheckpointResults,
	}
	if snapshot.CurrentStep != nil {
		value["currentStep"] = snapshot.CurrentStep
	}
	if err := validateRuntimeProtocol("RuntimeFrame", value); err != nil {
		return err
	}
	if snapshot.Tick > int64(jsonx.IntFromAny(scaleLimit["max_tick"])) {
		return frameError("快照时刻超过包声明上限")
	}
	if err := validateBackendStateScale(snapshot.State, scaleLimit); err != nil {
		return frameError(err.Error())
	}
	return validateTeachingFrameSemantics(snapshot.View, scaleLimit)
}

// validatePreviewFrames 确认审核依据非空且符合既有快照协议，不以成功徽章替代真实教学帧。
func validatePreviewFrames(frames []BackendSnapshot, scaleLimit map[string]any) error {
	if len(frames) == 0 || len(frames) > maxPreviewFrames {
		return frameError("样例教学帧数量必须为1到32个")
	}
	for i, frame := range frames {
		if err := validateBackendSnapshot(frame, scaleLimit); err != nil {
			return fmt.Errorf("校验第%d个样例教学帧失败: %w", i+1, err)
		}
	}
	return nil
}

// 结构先经共享 schema 校验；这里仅处理引用与包声明的动态容量。
func validateTeachingFrameSemantics(view map[string]any, scaleLimit map[string]any) error {
	layout := view["layout"].(map[string]any)
	patterns := view["patterns"].([]any)
	patternIDs := make(map[string]struct{}, len(patterns))
	for _, item := range patterns {
		pattern := item.(map[string]any)
		id := jsonx.StringField(pattern, "id")
		if _, exists := patternIDs[id]; exists {
			return frameError("视图编号重复")
		}
		if err := validatePatternScale(pattern, scaleLimit); err != nil {
			return err
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

func validateBackendDescriptor(descriptor BackendDescriptor, code, version string, schema InteractionSchema) error {
	raw, err := json.Marshal(descriptor)
	if err != nil {
		return frameError(fmt.Sprintf("编码包描述失败: %v", err))
	}
	var value any
	if err := jsonx.DecodeStrict(raw, &value); err != nil {
		return frameError(fmt.Sprintf("读取包描述失败: %v", err))
	}
	if err := validateRuntimeProtocol("SimPackageDescriptor", value); err != nil {
		return err
	}
	if jsonx.StringField(descriptor.Meta, "code") != code || jsonx.StringField(descriptor.Meta, "version") != version {
		return frameError("包自描述信息的编号或版本与已登记的包不一致")
	}
	var controls struct {
		Interactions []struct {
			simInteractionDef
			ElementFilter string `json:"elementFilter"`
			LabelTag      string `json:"labelTag"`
		} `json:"interactions"`
	}
	if err := jsonx.DecodeStrict(raw, &controls); err != nil {
		return frameError(fmt.Sprintf("读取控件声明失败: %v", err))
	}
	if len(controls.Interactions) != len(schema.Events) {
		return frameError("包自描述的交互数量与审核声明不一致")
	}
	seenEvents := make(map[string]bool, len(controls.Interactions))
	seenIDs := make(map[string]bool, len(controls.Interactions))
	for _, control := range controls.Interactions {
		in := control.simInteractionDef
		in.ElementFilter, in.LabelTag = control.ElementFilter, control.LabelTag
		actual, findings := interactionSchemaFromManifest(in)
		expected, ok := schema.Events[in.Emits]
		if !ok || len(findings) > 0 || in.ID != expected.InteractionID ||
			actual.Kind != expected.Kind || actual.Target != expected.Target ||
			!slices.EqualFunc(actual.Params, expected.Params, func(a, b InteractionParam) bool { return jsonvalue.Equal(a, b) }) {
			return frameError("运行控件与审核声明不一致: " + in.Emits)
		}
		if seenEvents[in.Emits] || seenIDs[in.ID] {
			return frameError("包自描述信息包含重复的交互事件或控件编号: " + in.Emits)
		}
		seenEvents[in.Emits], seenIDs[in.ID] = true, true
	}
	return nil
}

func validatePatternScale(pattern map[string]any, limits map[string]any) error {
	data := pattern["data"].(map[string]any)
	array := func(key string) []any { return data[key].([]any) }
	nodes, events := 0, 0
	switch pattern["mode"] {
	case "graph":
		nodes, events = len(array("nodes")), len(array("edges"))
	case "chain":
		nodes = len(array("blocks"))
		for _, fork := range array("forks") {
			nodes += len(fork.([]any))
		}
	case "tree":
		queue := []any{data["root"]}
		for len(queue) > 0 {
			current := queue[len(queue)-1].(map[string]any)
			queue = queue[:len(queue)-1]
			nodes++
			if children, ok := current["children"].([]any); ok {
				queue = append(queue, children...)
			}
		}
	case "matrix":
		rows, columns, cells := array("rows"), array("columns"), array("cells")
		nodes, events = max(len(rows), len(columns)), len(rows)*len(columns)
		if len(cells) != len(rows) {
			return frameError("矩阵行数据不完整")
		}
		for _, row := range cells {
			if len(row.([]any)) != len(columns) {
				return frameError("矩阵列数据不完整")
			}
		}
	case "pipeline":
		nodes = len(array("steps"))
	case "lane":
		actors := make(map[string]bool)
		for _, actor := range array("actors") {
			actors[actor.(string)] = true
		}
		nodes, events = len(array("actors")), len(array("messages"))
		for _, value := range array("messages") {
			message := value.(map[string]any)
			if !actors[message["from"].(string)] || !actors[message["to"].(string)] {
				return frameError("时序消息引用了未声明的参与方")
			}
		}
	case "chart":
		nodes = len(array("series"))
		for _, series := range array("series") {
			events += len(series.(map[string]any)["points"].([]any))
		}
	}
	if nodes > jsonx.IntFromAny(limits["nodes"]) || events > jsonx.IntFromAny(limits["max_events"]) {
		return frameError("视图超过包声明的节点或事件规模")
	}
	return nil
}
