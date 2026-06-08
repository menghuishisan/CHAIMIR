// M2 沙箱进度事件与 topic 约定。
// progress WS 是模块自有实时流,topic 命名沿用总蓝图的 <模块>:<资源id>:<频道>。
package sandbox

import (
	"encoding/json"
	"fmt"
)

// progressTopic 返回某个沙箱进度推送的 topic。
func progressTopic(sandboxID int64) string {
	return fmt.Sprintf("sandbox:%d:progress", sandboxID)
}

// progressPayload 把进度事件编码为 WS 广播载荷。
func progressPayload(event SandboxProgressEvent) []byte {
	data, err := json.Marshal(event)
	if err != nil {
		return []byte(`{"message":"progress encode failed"}`)
	}
	return data
}
