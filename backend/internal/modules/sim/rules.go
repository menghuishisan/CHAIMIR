// sim rules 文件定义 M4 纯输入校验、状态机和审核规则,不访问 repo/db/contracts。
package sim

import (
	"encoding/hex"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	"chaimir/internal/platform/auth"
	"chaimir/internal/platform/jsonx"
	"chaimir/pkg/apperr"
	"chaimir/pkg/privacy"
)

var (
	simCodePattern        = regexp.MustCompile(`^[a-z][a-z0-9_]{1,31}__[a-z0-9][a-z0-9-]{1,62}[a-z0-9]$`)
	semverPattern         = regexp.MustCompile(`^[0-9]+\.[0-9]+\.[0-9]+(?:[-+][0-9A-Za-z.-]+)?$`)
	categoryPattern       = regexp.MustCompile(`^[a-z][a-z0-9_-]{1,31}$`)
	eventTypePattern      = regexp.MustCompile(`^[a-z][a-z0-9_.:-]{0,63}$`)
	checkpointIDPattern   = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_.:-]{0,95}$`)
	payloadKeyPattern     = regexp.MustCompile(`^[A-Za-z][A-Za-z0-9_.:-]{0,63}$`)
	immutableReportKeys   = map[string]struct{}{"static_scan": {}, "bundle_hash": {}, "metadata_validation": {}}
	dynamicReportKeyAllow = map[string]struct{}{"determinism_check": {}, "worker_preview": {}, "details": {}}
)

const (
	maxActionPayloadBytes = 16384
	maxPublicStringLength = 512
)

// computeFromString 将接口字符串转换为数据库枚举。
func computeFromString(value string) (int16, error) {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "", "frontend":
		return ComputeFrontend, nil
	case "backend":
		return ComputeBackend, nil
	default:
		return 0, apperr.ErrSimPackageInvalid
	}
}

// computeText 返回 API 对外稳定字符串。
func computeText(value int16) (string, error) {
	switch value {
	case ComputeFrontend:
		return "frontend", nil
	case ComputeBackend:
		return "backend", nil
	default:
		return "", apperr.ErrSimPackageDataCorrupt.WithCause(fmt.Errorf("仿真包计算模式异常: compute=%d", value))
	}
}

// userPackageListStatus 校验用户侧包列表只能查询已上架状态。
func userPackageListStatus(value string) (int16, error) {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "", "published":
		return PackageStatusPublished, nil
	default:
		return 0, apperr.ErrQueryParamInvalid
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
func normalizePackageRequest(req SubmitPackageRequest) (SubmitPackageRequest, int16, error) {
	req.Code = strings.TrimSpace(req.Code)
	req.Version = strings.TrimSpace(req.Version)
	req.Name = strings.TrimSpace(req.Name)
	req.Category = strings.TrimSpace(req.Category)
	req.BackendAdapter = strings.TrimSpace(req.BackendAdapter)
	compute, err := computeFromString(req.Compute)
	if err != nil {
		return req, 0, err
	}
	if len(req.ScaleLimit) == 0 {
		req.ScaleLimit = []byte(`{}`)
	}
	if len(req.BackendConfig) == 0 {
		req.BackendConfig = []byte(`{}`)
	}
	return req, compute, nil
}

// validatePackageRequest 校验仿真包元数据和命名空间边界。
func validatePackageRequest(req SubmitPackageRequest, compute int16, authorID int64) error {
	if !simCodePattern.MatchString(req.Code) || !semverPattern.MatchString(req.Version) || strings.TrimSpace(req.Name) == "" || !categoryPattern.MatchString(req.Category) {
		return apperr.ErrSimPackageInvalid
	}
	if len(req.Name) > 128 || len(req.Code) > 96 || len(req.Version) > 32 {
		return apperr.ErrSimPackageInvalid
	}
	if !jsonObject(req.ScaleLimit) || !jsonObject(req.BackendConfig) {
		return apperr.ErrSimPackageInvalid
	}
	if compute == ComputeBackend && strings.TrimSpace(req.BackendAdapter) == "" {
		return apperr.ErrSimPackageInvalid
	}
	if compute == ComputeFrontend && strings.TrimSpace(req.BackendAdapter) != "" {
		return apperr.ErrSimPackageInvalid
	}
	if compute == ComputeFrontend && !jsonObjectEmpty(req.BackendConfig) {
		return apperr.ErrSimPackageInvalid
	}
	if authorID <= 0 || !strings.HasPrefix(req.Code, "teacher_"+strconv.FormatInt(authorID, 10)+"__") {
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

// validateAction 校验操作序列内容。
func validateAction(req ReportActionRequest) error {
	if req.Seq <= 0 || req.AtTick < 0 || !eventTypePattern.MatchString(strings.TrimSpace(req.EventType)) {
		return apperr.ErrSimActionSeqInvalid
	}
	if req.Payload == nil {
		return nil
	}
	raw, err := jsonx.AnyBytes(req.Payload, apperr.ErrSimActionSeqInvalid)
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

// validateDynamicReport 确保 validation-report 不能覆盖后端生成的静态安全字段。
func validateDynamicReport(raw map[string]any) error {
	for key := range raw {
		if _, blocked := immutableReportKeys[key]; blocked {
			return apperr.ErrSimPackageValidationFailed
		}
		if _, ok := dynamicReportKeyAllow[key]; !ok {
			return apperr.ErrSimPackageValidationFailed
		}
	}
	return nil
}

// validateValidationReportRequest 强制受控预览报告只能写入标准结构和值域。
func validateValidationReportRequest(req ValidationReportRequest) error {
	if err := validateValidationStatus(req.DeterminismCheck); err != nil {
		return err
	}
	if err := validateValidationStatus(req.WorkerPreview); err != nil {
		return err
	}
	if len(req.Details) > 32 {
		return apperr.ErrSimPackageValidationFailed
	}
	for key, value := range req.Details {
		if !payloadKeyPattern.MatchString(strings.TrimSpace(key)) || strings.TrimSpace(value) == "" || len(value) > 500 {
			return apperr.ErrSimPackageValidationFailed
		}
	}
	return nil
}

// validateValidationStatus 限定动态审核子项枚举和用户可见摘要长度。
func validateValidationStatus(status ValidationStatus) error {
	switch strings.TrimSpace(status.Status) {
	case validationPassed, validationFailed:
	default:
		return apperr.ErrSimPackageValidationFailed
	}
	if len(strings.TrimSpace(status.Message)) > 500 {
		return apperr.ErrSimPackageValidationFailed
	}
	return nil
}

// validateApprovalReport 校验审核通过所需的后端静态和受控预览门禁。
func validateApprovalReport(report ValidationReport, pkg Package) error {
	if report.MetadataValidation.Status != validationPassed || report.StaticScan.Status != validationPassed || report.DeterminismCheck.Status != validationPassed || report.WorkerPreview.Status != validationPassed {
		return apperr.ErrSimPackageValidationFailed
	}
	if !isSHA256Hex(report.BundleHash) || report.BundleHash != pkg.BundleHash {
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
func validateActionAgainstSchema(schema InteractionSchema, req ReportActionRequest) error {
	schema = normalizeInteractionSchema(schema)
	event, ok := schema.Events[strings.TrimSpace(req.EventType)]
	if !ok {
		return apperr.ErrSimActionSeqInvalid
	}
	payload := req.Payload
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
		text, ok := stringFromPayload(value)
		if !ok {
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

// jsonObjectEmpty 校验字段是空 JSON 对象。
func jsonObjectEmpty(raw []byte) bool {
	var value map[string]any
	return len(raw) > 0 && jsonx.DecodeStrict(raw, &value) == nil && len(value) == 0
}

// isSHA256Hex 校验内容哈希格式。
func isSHA256Hex(value string) bool {
	value = strings.TrimSpace(value)
	if len(value) != 64 {
		return false
	}
	_, err := hex.DecodeString(value)
	return err == nil
}
