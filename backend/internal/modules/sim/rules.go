// sim rules 文件定义 M4 纯输入校验、状态机和审核规则,不访问数据库或外部服务。
package sim

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"time"

	"chaimir/internal/contracts"
	"chaimir/internal/platform/auth"
	"chaimir/internal/platform/ids"
	"chaimir/internal/platform/jsonx"
	"chaimir/pkg/apperr"
	pkgcrypto "chaimir/pkg/crypto"
	"chaimir/pkg/jsonvalue"
	"chaimir/pkg/privacy"
)

var (
	simCodePattern    = regexp.MustCompile(`^[a-z][a-z0-9_]{1,31}__[a-z0-9][a-z0-9-]{1,62}[a-z0-9]$`)
	semverPattern     = regexp.MustCompile(`^[0-9]+\.[0-9]+\.[0-9]+(?:[-+][0-9A-Za-z.-]+)?$`)
	categoryPattern   = regexp.MustCompile(`^[a-z][a-z0-9_-]{1,31}$`)
	eventTypePattern  = regexp.MustCompile(`^[a-z][a-z0-9_.:-]{0,63}$`)
	payloadKeyPattern = regexp.MustCompile(`^[A-Za-z][A-Za-z0-9_.:-]{0,63}$`)
)

// M4 协议与入库边界的规模上限统一放在这里:它们是协议常量(前后端同值、与数据库约束同源),
// 不是部署可调阈值,故不进 config;分散到各校验文件会让"同一类上限在哪"变成翻文件游戏。
const (
	maxActionPayloadBytes = 16384
	maxPublicStringLength = 512
	// maxValidationMessageLength 限定审核报告里展示给包作者的单条原因长度。
	// 原因文本来自隔离容器的输出,长度不可信,必须在入库前截断。
	maxValidationMessageLength = 500
	// maxPreviewFrames 是隔离预览可写入报告的样例帧数量上限,兜住报告体积。
	maxPreviewFrames = 32
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

func normalizePackageRequest(req SubmitPackageRequest) SubmitPackageRequest {
	req.Code = strings.TrimSpace(req.Code)
	req.Version = strings.TrimSpace(req.Version)
	req.Name = strings.TrimSpace(req.Name)
	req.Category = strings.TrimSpace(req.Category)
	return req
}

func validatePackageRequest(req SubmitPackageRequest, authorID int64) (map[string]any, error) {
	if !simCodePattern.MatchString(req.Code) || !semverPattern.MatchString(req.Version) || strings.TrimSpace(req.Name) == "" || !categoryPattern.MatchString(req.Category) {
		return nil, apperr.ErrSimPackageInvalid
	}
	if len(req.Name) > 128 || len(req.Code) > 96 || len(req.Version) > 32 {
		return nil, apperr.ErrSimPackageInvalid
	}
	scaleLimit, err := jsonx.ObjectMapStrict(req.ScaleLimit)
	if err != nil || !validManifestScaleLimit(scaleLimit) {
		return nil, apperr.ErrSimPackageInvalid
	}
	if authorID <= 0 || !strings.HasPrefix(req.Code, "teacher_"+ids.Format(authorID)+"__") {
		return nil, apperr.ErrSimPackageInvalid
	}
	return scaleLimit, nil
}

// validateCreateSession 校验内部会话创建请求。
func validateCreateSession(req contracts.SimCreateSessionRequest) error {
	if req.TenantID <= 0 || !simCodePattern.MatchString(strings.TrimSpace(req.PackageCode)) || !semverPattern.MatchString(strings.TrimSpace(req.Version)) || req.OwnerAccountID <= 0 || !auth.ValidSourceRef(req.SourceRef) || !auth.ValidScopeRef(req.ScopeRef) {
		return apperr.ErrSimSessionInvalid
	}
	return nil
}

// 时刻推进不单独入库；重放到这条用户操作共需 atTick + seq 条事件。
func validateActionReplayPosition(seq, atTick int32, scaleLimit map[string]any) error {
	if seq <= 0 || atTick < 0 || int64(atTick) > int64(jsonx.IntFromAny(scaleLimit["max_tick"])) || int64(atTick)+int64(seq) > int64(jsonx.IntFromAny(scaleLimit["max_events"])) {
		return apperr.ErrSimActionSeqInvalid
	}
	return nil
}

// normalizeSessionSharedAccountIDs 生成会话创建事务要写入的唯一非 owner 共享账号集合。
func normalizeSessionSharedAccountIDs(ownerAccountID int64, authorizedAccountIDs []int64) ([]int64, error) {
	if ownerAccountID <= 0 {
		return nil, apperr.ErrSimSessionInvalid
	}
	seen := map[int64]struct{}{ownerAccountID: {}}
	out := make([]int64, 0, len(authorizedAccountIDs))
	for _, accountID := range authorizedAccountIDs {
		if accountID <= 0 {
			return nil, apperr.ErrSimSessionInvalid
		}
		if _, exists := seen[accountID]; exists {
			continue
		}
		seen[accountID] = struct{}{}
		out = append(out, accountID)
	}
	return out, nil
}

// sessionAccountAuthorized 判断账号是否为会话 owner 或创建时明确记录的共享成员。
func sessionAccountAuthorized(session Session, accountID int64) bool {
	if accountID <= 0 {
		return false
	}
	if session.OwnerAccountID == accountID {
		return true
	}
	for _, sharedAccountID := range session.SharedAccountIDs {
		if sharedAccountID == accountID {
			return true
		}
	}
	return false
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

// validateApprovalReport 校验上架所需的真实报告；非阻断扫描提示保留给人工判断。
func validateApprovalReport(report ValidationReport, pkg Package) error {
	if report.MetadataValidation.Status != validationPassed || report.DeterminismCheck.Status != validationPassed || report.WorkerPreview.Status != validationPassed {
		return apperr.ErrSimPackageValidationFailed
	}
	scan := report.StaticScan
	if (scan.Status != validationPassed && (scan.Status != validationWarning || len(scan.Findings) == 0)) || hasBlockingFindings(scan.Findings) {
		return apperr.ErrSimPackageValidationFailed
	}
	if !pkgcrypto.ValidSHA256Hex(report.BundleHash) || report.BundleHash != pkg.BundleHash {
		return apperr.ErrSimPackageValidationFailed
	}
	var frames []BackendSnapshot
	if err := jsonx.DecodeStrictKnownFields(report.PreviewFrames, &frames); err != nil {
		return apperr.ErrSimPackageValidationFailed.WithCause(err)
	}
	if err := validatePreviewFrames(frames, pkg.ScaleLimit); err != nil {
		return apperr.ErrSimPackageValidationFailed.WithCause(err)
	}
	return nil
}

// actionEqual 判断重复 seq 的内容是否完全相同,用于幂等上报。
func actionEqual(existing Action, req ReportActionRequest) (bool, error) {
	if existing.Seq != req.Seq || existing.AtTick != req.AtTick || existing.EventType != strings.TrimSpace(req.EventType) {
		return false, nil
	}
	return jsonvalue.Equal(existing.Payload, req.Payload), nil
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
	case bool, float64, int, int32, int64, json.Number:
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
