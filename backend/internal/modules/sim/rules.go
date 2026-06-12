// sim rules 文件定义 M4 纯输入校验、状态机和审核规则,不访问 repo/db/contracts。
package sim

import (
	"encoding/hex"
	"encoding/json"
	"regexp"
	"strconv"
	"strings"
	"time"

	"chaimir/internal/platform/auth"
	"chaimir/pkg/apperr"
)

var (
	simCodePattern        = regexp.MustCompile(`^[a-z][a-z0-9_]{1,31}__[a-z0-9][a-z0-9-]{1,62}[a-z0-9]$`)
	semverPattern         = regexp.MustCompile(`^[0-9]+\.[0-9]+\.[0-9]+(?:[-+][0-9A-Za-z.-]+)?$`)
	categoryPattern       = regexp.MustCompile(`^[a-z][a-z0-9_-]{1,31}$`)
	eventTypePattern      = regexp.MustCompile(`^[a-z][a-z0-9_.:-]{0,63}$`)
	checkpointIDPattern   = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_.:-]{0,95}$`)
	immutableReportKeys   = map[string]struct{}{"static_scan": {}, "bundle_hash": {}, "metadata_validation": {}}
	dynamicReportKeyAllow = map[string]struct{}{"determinism_check": {}, "worker_preview": {}, "details": {}}
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
func computeText(value int16) string {
	switch value {
	case ComputeBackend:
		return "backend"
	default:
		return "frontend"
	}
}

// packageStatusFromQuery 解析列表状态过滤条件。
func packageStatusFromQuery(value string) int16 {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "draft":
		return PackageStatusDraft
	case "reviewing", "pending":
		return PackageStatusReviewing
	case "", "published":
		return PackageStatusPublished
	case "archived":
		return PackageStatusArchived
	case "rejected":
		return PackageStatusRejected
	default:
		return 0
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

// reviewResultFromQuery 解析审核列表状态过滤条件。
func reviewResultFromQuery(value string) int16 {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "", "pending":
		return ReviewPending
	case "approved":
		return ReviewApproved
	case "rejected":
		return ReviewRejected
	default:
		return 0
	}
}

// normalizePackageRequest 修剪字段并给空 JSON 字段补默认对象。
func normalizePackageRequest(req SubmitPackageRequest, fallbackAuthorType int16) (SubmitPackageRequest, int16, error) {
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
		req.ScaleLimit = json.RawMessage(`{}`)
	}
	if len(req.BackendConfig) == 0 {
		req.BackendConfig = json.RawMessage(`{}`)
	}
	if req.AuthorType == 0 {
		req.AuthorType = fallbackAuthorType
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
	switch req.AuthorType {
	case AuthorPlatformBuiltIn:
		if !strings.HasPrefix(req.Code, "builtin__") {
			return apperr.ErrSimPackageInvalid
		}
	case AuthorTeacher:
		if authorID <= 0 || !strings.HasPrefix(req.Code, "teacher_"+strconv.FormatInt(authorID, 10)+"__") {
			return apperr.ErrSimPackageInvalid
		}
	case AuthorThirdParty:
		if !strings.HasPrefix(req.Code, "org_") || !strings.Contains(req.Code, "__") {
			return apperr.ErrSimPackageInvalid
		}
	default:
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
	return nil
}

// validateCheckpoint 校验检查点上报内容。
func validateCheckpoint(sessionID int64, checkpointID string, answer json.RawMessage) error {
	if sessionID <= 0 || !checkpointIDPattern.MatchString(strings.TrimSpace(checkpointID)) || len(answer) == 0 || !json.Valid(answer) {
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

// validateApprovalReport 校验审核通过所需的四项安全门禁。
func validateApprovalReport(report ValidationReport) error {
	if report.MetadataValidation.Status != validationPassed || report.StaticScan.Status != validationPassed || report.DeterminismCheck.Status != validationPassed || report.WorkerPreview.Status != validationPassed {
		return apperr.ErrSimPackageValidationFailed
	}
	if !isSHA256Hex(report.BundleHash) {
		return apperr.ErrSimPackageValidationFailed
	}
	return nil
}

// actionEqual 判断重复 seq 的内容是否完全相同,用于幂等上报。
func actionEqual(existing Action, req ReportActionRequest) (bool, error) {
	if existing.Seq != req.Seq || existing.AtTick != req.AtTick || existing.EventType != strings.TrimSpace(req.EventType) {
		return false, nil
	}
	existingRaw, err := json.Marshal(existing.Payload)
	if err != nil {
		return false, apperr.ErrSimActionSeqInvalid.WithCause(err)
	}
	reqRaw, err := json.Marshal(req.Payload)
	if err != nil {
		return false, apperr.ErrSimActionSeqInvalid.WithCause(err)
	}
	return string(existingRaw) == string(reqRaw), nil
}

// shareUsable 判断分享码是否仍可公开读取。
func shareUsable(share Share, now time.Time) bool {
	if share.Status != ShareActive {
		return false
	}
	return share.ExpireAt.IsZero() || now.Before(share.ExpireAt)
}

// jsonObject 校验字段是 JSON 对象,避免数组或标量破坏 SDK 契约。
func jsonObject(raw json.RawMessage) bool {
	var value map[string]any
	return len(raw) > 0 && json.Unmarshal(raw, &value) == nil
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
