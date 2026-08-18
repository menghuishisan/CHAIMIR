// sim rules 文件定义 M4 纯输入校验、状态机和审核规则,不访问 repo/db/contracts。
package sim

import (
	"fmt"
	"regexp"
	"strings"
	"time"

	"chaimir/internal/platform/auth"
	"chaimir/internal/platform/ids"
	"chaimir/internal/platform/jsonx"
	"chaimir/pkg/apperr"
	pkgcrypto "chaimir/pkg/crypto"
	"chaimir/pkg/privacy"
)

var (
	simCodePattern      = regexp.MustCompile(`^[a-z][a-z0-9_]{1,31}__[a-z0-9][a-z0-9-]{1,62}[a-z0-9]$`)
	semverPattern       = regexp.MustCompile(`^[0-9]+\.[0-9]+\.[0-9]+(?:[-+][0-9A-Za-z.-]+)?$`)
	categoryPattern     = regexp.MustCompile(`^[a-z][a-z0-9_-]{1,31}$`)
	eventTypePattern    = regexp.MustCompile(`^[a-z][a-z0-9_.:-]{0,63}$`)
	checkpointIDPattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_.:-]{0,95}$`)
	payloadKeyPattern   = regexp.MustCompile(`^[A-Za-z][A-Za-z0-9_.:-]{0,63}$`)
)

// M4 协议与入库边界的规模上限统一放在这里:它们是协议常量(前后端同值、与数据库约束同源),
// 不是部署可调阈值,故不进 config;分散到各校验文件会让"同一类上限在哪"变成翻文件游戏。
const (
	maxActionPayloadBytes = 16384
	maxPublicStringLength = 512
	// maxValidationMessageLength 限定审核报告里展示给包作者的单条原因长度。
	// 原因文本来自隔离容器的输出,长度不可信,必须在入库前截断。
	maxValidationMessageLength = 500
	// maxFramePatterns 是单帧允许的封闭模式数量上限,与前端 assertValidTeachingFrame 同值。
	maxFramePatterns = 3
	// maxPreviewFrames 是隔离预览可写入报告的样例帧数量上限,兜住报告体积。
	maxPreviewFrames = 32
	// maxDescriptorInteractions 限定页面一次渲出的操作数量上限。
	maxDescriptorInteractions = 64
	// maxDescriptorNarrative 限定教学步骤数量上限。
	maxDescriptorNarrative = 64
	// maxDescriptorCheckpoints 限定检查点数量上限。
	maxDescriptorCheckpoints = 64
)

// computeText 返回 API 对外稳定字符串。
// 只在响应里出现:执行位置由服务端按 author_type 派生,客户端不提交该字段,故没有反向解析。
func computeText(value int16) (string, error) {
	switch value {
	case ComputeBrowser:
		return "browser", nil
	case ComputeIsolated:
		return "isolated", nil
	default:
		return "", apperr.ErrSimPackageDataCorrupt.WithCause(fmt.Errorf("仿真包执行位置异常: compute=%d", value))
	}
}

// packageStatusFromText 把接口状态字符串转换为数据库枚举,与 packageStatusText 互为逆映射。
func packageStatusFromText(value string) (int16, error) {
	switch value {
	case "draft":
		return PackageStatusDraft, nil
	case "reviewing":
		return PackageStatusReviewing, nil
	case "published":
		return PackageStatusPublished, nil
	case "archived":
		return PackageStatusArchived, nil
	case "rejected":
		return PackageStatusRejected, nil
	default:
		return 0, apperr.ErrQueryParamInvalid
	}
}

// packageListStatus 校验包列表的状态过滤条件。
// 浏览可用场景(mine=false)只能查已上架:未审核与已下架的包不对使用者开放;
// 查询本人提交的包(mine=true)可按任一状态过滤,不传则返回本人全部状态的包(0 = 不过滤),
// 教师需要看到退回与草稿才能继续修改。
func packageListStatus(value string, mine bool) (int16, error) {
	normalized := strings.ToLower(strings.TrimSpace(value))
	if !mine {
		switch normalized {
		case "", "published":
			return PackageStatusPublished, nil
		default:
			return 0, apperr.ErrQueryParamInvalid
		}
	}
	if normalized == "" {
		return 0, nil
	}
	return packageStatusFromText(normalized)
}

// mineFlagFromQuery 解析「只看我提交的」过滤开关,非法取值显式报错。
func mineFlagFromQuery(value string) (bool, error) {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "", "false":
		return false, nil
	case "true":
		return true, nil
	default:
		return false, apperr.ErrQueryParamInvalid
	}
}

// reviewResultFromQuery 解析审核列表状态过滤条件,非法枚举显式报错。
func reviewResultFromQuery(value string) (int16, error) {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "", "pending":
		return ReviewPending, nil
	case "approved":
		return ReviewApproved, nil
	case "rejected":
		return ReviewRejected, nil
	default:
		return 0, apperr.ErrQueryParamInvalid
	}
}

// normalizePackageRequest 修剪字段并给空 JSON 字段补默认对象。
func normalizePackageRequest(req SubmitPackageRequest) SubmitPackageRequest {
	req.Code = strings.TrimSpace(req.Code)
	req.Version = strings.TrimSpace(req.Version)
	req.Name = strings.TrimSpace(req.Name)
	req.Category = strings.TrimSpace(req.Category)
	if len(req.ScaleLimit) == 0 {
		req.ScaleLimit = []byte(`{}`)
	}
	return req
}

// validatePackageRequest 校验仿真包元数据和命名空间边界。
// 执行位置与运行能力不在此校验:它们由服务端按登录账号派生,请求结构里不存在这两个字段。
func validatePackageRequest(req SubmitPackageRequest, authorID int64) error {
	if !simCodePattern.MatchString(req.Code) || !semverPattern.MatchString(req.Version) || strings.TrimSpace(req.Name) == "" || !categoryPattern.MatchString(req.Category) {
		return apperr.ErrSimPackageInvalid
	}
	if len(req.Name) > 128 || len(req.Code) > 96 || len(req.Version) > 32 {
		return apperr.ErrSimPackageInvalid
	}
	if !jsonObject(req.ScaleLimit) {
		return apperr.ErrSimPackageInvalid
	}
	if authorID <= 0 || !strings.HasPrefix(req.Code, "teacher_"+ids.Format(authorID)+"__") {
		return apperr.ErrSimPackageInvalid
	}
	return nil
}

// validateCreateSession 校验内部会话创建请求。
func validateCreateSession(req CreateSessionRequest, tenantID int64) error {
	if tenantID <= 0 || !simCodePattern.MatchString(strings.TrimSpace(req.PackageCode)) || !semverPattern.MatchString(strings.TrimSpace(req.Version)) || req.OwnerAccountID <= 0 || !auth.ValidSourceRef(req.SourceRef) {
		return apperr.ErrSimSessionInvalid
	}
	if req.InitParams == nil {
		req.InitParams = map[string]any{}
	}
	return nil
}

// validateAction 校验客户端上报的操作序列内容。
func validateAction(req ReportActionRequest) error {
	if req.Seq <= 0 || req.AtTick < 0 {
		return apperr.ErrSimActionSeqInvalid
	}
	return validateActionContent(req.EventType, req.Payload)
}

// validateActionContent 校验一次操作的事件名与 payload 体积。
// 隔离执行连接单独用它:那里的序号由仓储在事务内续号,连接侧没有也不该编一个序号出来。
func validateActionContent(eventType string, payload map[string]any) error {
	if !eventTypePattern.MatchString(strings.TrimSpace(eventType)) {
		return apperr.ErrSimActionSeqInvalid
	}
	if payload == nil {
		return nil
	}
	raw, err := jsonx.AnyBytes(payload, apperr.ErrSimActionSeqInvalid)
	if err != nil || len(raw) > maxActionPayloadBytes {
		return apperr.ErrSimActionSeqInvalid
	}
	return nil
}

// validateCheckpoint 校验检查点上报内容。
func validateCheckpoint(sessionID int64, checkpointID string, answer []byte) error {
	if sessionID <= 0 || !checkpointIDPattern.MatchString(strings.TrimSpace(checkpointID)) || len(answer) == 0 || !jsonx.Valid(answer) {
		return apperr.ErrSimCheckpointInvalid
	}
	return nil
}

// validateApprovalReport 校验审核通过所需的后端静态和受控预览门禁。
func validateApprovalReport(report ValidationReport, pkg Package) error {
	if report.MetadataValidation.Status != validationPassed || report.StaticScan.Status != validationPassed || report.DeterminismCheck.Status != validationPassed || report.WorkerPreview.Status != validationPassed {
		return apperr.ErrSimPackageValidationFailed
	}
	if !pkgcrypto.ValidSHA256Hex(report.BundleHash) || report.BundleHash != pkg.BundleHash {
		return apperr.ErrSimPackageValidationFailed
	}
	return nil
}

// actionEqual 判断重复 seq 的内容是否完全相同,用于幂等上报。
func actionEqual(existing Action, req ReportActionRequest) (bool, error) {
	if existing.Seq != req.Seq || existing.AtTick != req.AtTick || existing.EventType != strings.TrimSpace(req.EventType) {
		return false, nil
	}
	return jsonx.Equal(existing.Payload, req.Payload), nil
}

// validateActionAgainstSchema 按包内交互白名单校验用户操作,拒绝未声明事件和多余字段。
func validateActionAgainstSchema(schema InteractionSchema, eventType string, rawPayload map[string]any) error {
	schema = normalizeInteractionSchema(schema)
	event, ok := schema.Events[strings.TrimSpace(eventType)]
	if !ok {
		return apperr.ErrSimActionSeqInvalid
	}
	payload := rawPayload
	if payload == nil {
		payload = map[string]any{}
	}
	target, hasTarget := payload["target"]
	if event.Target == "element" {
		if !hasTarget || strings.TrimSpace(jsonx.StringFromAny(target)) == "" || len(jsonx.StringFromAny(target)) > 128 {
			return apperr.ErrSimActionSeqInvalid
		}
	} else if hasTarget {
		return apperr.ErrSimActionSeqInvalid
	}
	for key := range payload {
		if key == "target" {
			continue
		}
		if !payloadKeyPattern.MatchString(strings.TrimSpace(key)) {
			return apperr.ErrSimActionSeqInvalid
		}
		if platformPayloadValueMatchesInteraction(key, payload[key], event) {
			continue
		}
		param, ok := event.ParamIndex[key]
		if !ok || !payloadValueMatchesParam(payload[key], param) {
			return apperr.ErrSimActionSeqInvalid
		}
	}
	for _, param := range event.Params {
		if param.Required {
			if _, ok := payload[param.Name]; !ok {
				return apperr.ErrSimActionSeqInvalid
			}
		}
	}
	return nil
}

// platformPayloadValueMatchesInteraction 校验通用交互控件自动生成的固定字段,其余字段仍必须来自 manifest params。
func platformPayloadValueMatchesInteraction(key string, value any, event InteractionEventSchema) bool {
	switch key {
	case "active":
		_, ok := value.(bool)
		return event.Kind == "hold" && ok
	case "phase":
		text, ok := stringFromPayload(value)
		return event.Kind == "drag" && ok && (text == "start" || text == "move" || text == "end")
	case "startX", "startY", "currentX", "currentY", "deltaX", "deltaY":
		_, ok := jsonx.Float64FromAnyOK(value)
		return event.Kind == "drag" && ok
	default:
		return false
	}
}

// payloadValueMatchesParam 校验字段值与 manifest FieldDef 一致。
func payloadValueMatchesParam(value any, param InteractionParam) bool {
	switch param.Type {
	case "number", "range":
		n, ok := jsonx.Float64FromAnyOK(value)
		if !ok {
			return false
		}
		if param.Min != nil && n < *param.Min {
			return false
		}
		if param.Max != nil && n > *param.Max {
			return false
		}
		return true
	case "string":
		text, ok := stringFromPayload(value)
		return ok && len(text) <= maxPublicStringLength
	case "boolean":
		_, ok := value.(bool)
		return ok
	case "select":
		text := strings.TrimSpace(jsonx.StringFromAny(value))
		if text == "" {
			return false
		}
		for _, option := range param.Options {
			if text == option {
				return true
			}
		}
		return false
	default:
		return false
	}
}

// stringFromPayload 读取交互参数中的非空字符串值。
func stringFromPayload(value any) (string, bool) {
	text, ok := value.(string)
	if !ok {
		return "", false
	}
	text = strings.TrimSpace(text)
	return text, text != ""
}

// publicReplayMap 过滤公开分享剧本中的敏感字段,仅保留确定性复现所需公开参数。
func publicReplayMap(in map[string]any) map[string]any {
	return publicObject(in)
}

// publicObject 递归保留可公开复现的对象字段,过滤敏感或内部字段。
func publicObject(in map[string]any) map[string]any {
	out := map[string]any{}
	for key, value := range in {
		if !publicReplayKey(key) {
			continue
		}
		if public, ok := publicValue(value); ok {
			out[key] = public
		}
	}
	return out
}

// publicValue 限制公开分享参数的 JSON 类型和长度,避免分享码泄露大对象或内部结构。
func publicValue(value any) (any, bool) {
	switch v := value.(type) {
	case nil:
		return nil, true
	case bool, float64, int, int32, int64:
		return v, true
	case string:
		if len(v) > maxPublicStringLength {
			return "", false
		}
		return v, true
	case map[string]any:
		return publicObject(v), true
	case []any:
		if len(v) > 128 {
			return nil, false
		}
		out := make([]any, 0, len(v))
		for _, item := range v {
			clean, ok := publicValue(item)
			if !ok {
				return nil, false
			}
			out = append(out, clean)
		}
		return out, true
	default:
		return nil, false
	}
}

// publicReplayKey 统一复用 pkg/privacy 判断用户可见结果敏感字段。
func publicReplayKey(key string) bool {
	key = strings.ToLower(strings.TrimSpace(key))
	return key != "" && !strings.HasPrefix(key, "_") && !privacy.IsResultSensitiveKey(key) && payloadKeyPattern.MatchString(key)
}

// shareUsable 判断分享码是否仍可公开读取。
func shareUsable(share Share, now time.Time) bool {
	if share.Status != ShareActive {
		return false
	}
	return share.ExpireAt.IsZero() || now.Before(share.ExpireAt)
}

// canMutateSession 限制用户和内部服务只能修改活跃会话。
func canMutateSession(status int16) bool {
	switch status {
	case SessionCreating, SessionRunning, SessionIdle:
		return true
	default:
		return false
	}
}

// canArchiveSession 落地会话归档状态机,终态不能重复迁移。
func canArchiveSession(status int16) bool {
	switch status {
	case SessionCreating, SessionRunning, SessionIdle, SessionCompleted:
		return true
	default:
		return false
	}
}

// jsonObject 校验字段是 JSON 对象,避免数组或标量破坏 SDK 契约。
func jsonObject(raw []byte) bool {
	var value map[string]any
	return len(raw) > 0 && jsonx.DecodeStrict(raw, &value) == nil
}
