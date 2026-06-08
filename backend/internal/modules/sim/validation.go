// M4 校验逻辑:集中处理仿真包、会话、操作序列与 source_ref 的输入边界。
package sim

import (
	"context"
	"regexp"
	"strings"

	"chaimir/internal/modules/sim/internal/sqlcgen"
	"chaimir/internal/platform/auth"
	"chaimir/internal/platform/ids"
	"chaimir/internal/platform/jsonx"
	"chaimir/internal/platform/tenant"
	"chaimir/pkg/apperr"
)

var (
	semverRe      = regexp.MustCompile(`^[0-9]+\.[0-9]+\.[0-9]+(?:[-+][0-9A-Za-z.-]+)?$`)
	packageCodeRe = regexp.MustCompile(`^(builtin|teacher_[0-9]+|org_[0-9]+)__[a-z0-9][a-z0-9-]{1,80}$`)
	hashRe        = regexp.MustCompile(`^[a-fA-F0-9]{64}$`)
	sourceRefRe   = regexp.MustCompile(`^[a-z]+:[0-9]{4}:[a-z][a-z0-9-]*:[0-9A-Za-z_-]+$`)
	shareCodeRe   = regexp.MustCompile(`^[A-Za-z0-9_-]{16,48}$`)
)

// validateSubmitPackageRequest 校验仿真包提交请求,防止不稳定版本或命名空间冲突进入审核。
func validateSubmitPackageRequest(req SubmitPackageRequest) error {
	// 第一步校验插件身份字段,确保全平台 code 具备稳定命名空间。
	if !packageCodeRe.MatchString(strings.TrimSpace(req.Code)) || !semverRe.MatchString(strings.TrimSpace(req.Version)) ||
		strings.TrimSpace(req.Name) == "" || strings.TrimSpace(req.Category) == "" {
		return apperr.ErrSimPackageInvalid
	}
	// 第二步校验 bundle 引用与完整性 hash,保证审核和回放可追溯同一包内容。
	if strings.TrimSpace(req.BundleKey) == "" || !hashRe.MatchString(strings.TrimSpace(req.BundleHash)) {
		return apperr.ErrSimPackageInvalid
	}
	// 第三步按运行位置校验后端适配器,避免 compute=backend 形成无执行器配置。
	compute, err := parseCompute(req.Compute)
	if err != nil {
		return err
	}
	if compute == ComputeBackend && strings.TrimSpace(req.BackendAdapter) == "" {
		return apperr.ErrSimPackageInvalid
	}
	if req.AuthorType < AuthorTypeBuiltin || req.AuthorType > AuthorTypeOrg {
		return apperr.ErrSimPackageInvalid
	}
	if req.AuthorType != AuthorTypeBuiltin && strings.TrimSpace(req.AuthorID) == "" {
		return apperr.ErrSimPackageInvalid
	}
	if err := validatePackageNamespace(req); err != nil {
		return err
	}
	return nil
}

// validatePackageNamespace 绑定作者类型、作者 ID 与 code 前缀,防止扩展包覆盖内置包或冒用他人命名空间。
func validatePackageNamespace(req SubmitPackageRequest) error {
	code := strings.TrimSpace(req.Code)
	switch req.AuthorType {
	case AuthorTypeBuiltin:
		if !strings.HasPrefix(code, "builtin__") {
			return apperr.ErrSimPackageInvalid
		}
		return nil
	case AuthorTypeTeacher:
		authorID, ok := ids.Parse(req.AuthorID)
		if !ok || !strings.HasPrefix(code, "teacher_"+ids.Format(authorID)+"__") {
			return apperr.ErrSimPackageInvalid
		}
		return nil
	case AuthorTypeOrg:
		authorID, ok := ids.Parse(req.AuthorID)
		if !ok || !strings.HasPrefix(code, "org_"+ids.Format(authorID)+"__") {
			return apperr.ErrSimPackageInvalid
		}
		return nil
	default:
		return apperr.ErrSimPackageInvalid
	}
}

// validatePackageAuthorAccess 校验包维护权限,教师/组织作者只能维护自己命名空间下的包。
func validatePackageAuthorAccess(id tenant.Identity, pkg sqlcgen.SimPackage) error {
	if id.IsPlatform {
		return nil
	}
	if pkg.AuthorType == AuthorTypeTeacher || pkg.AuthorType == AuthorTypeOrg {
		if pkg.AuthorID.Valid && pkg.AuthorID.Int64 == id.AccountID {
			return nil
		}
	}
	return apperr.ErrSimAccessDenied
}

// validatePackageSubmitterAccess 校验提交者身份与 author_type/author_id 一致,防止冒充平台内置或他人命名空间。
func validatePackageSubmitterAccess(id tenant.Identity, req SubmitPackageRequest) error {
	if id.IsPlatform {
		return nil
	}
	if req.AuthorType != AuthorTypeTeacher {
		return apperr.ErrSimAccessDenied
	}
	authorID, ok := ids.Parse(req.AuthorID)
	if !ok || authorID != id.AccountID {
		return apperr.ErrSimAccessDenied
	}
	return nil
}

// validatePackageListStatusAccess 限制普通用户只能查看已上架包,避免枚举草稿和审核中资源。
func validatePackageListStatusAccess(id tenant.Identity, status int16) error {
	if id.IsPlatform || status == PackageStatusPublished {
		return nil
	}
	return apperr.ErrSimAccessDenied
}

// validateUpdatePackageRequest 校验可修改字段,已上架版本不可经本路径覆盖。
func validateUpdatePackageRequest(req UpdatePackageRequest, compute int16) error {
	if strings.TrimSpace(req.Name) == "" || strings.TrimSpace(req.Category) == "" ||
		strings.TrimSpace(req.BundleKey) == "" || !hashRe.MatchString(strings.TrimSpace(req.BundleHash)) {
		return apperr.ErrSimPackageInvalid
	}
	if compute == ComputeBackend && strings.TrimSpace(req.BackendAdapter) == "" {
		return apperr.ErrSimPackageInvalid
	}
	return nil
}

// validateCreateSessionRequest 校验创建会话参数,租户和 owner 由服务层补齐。
func validateCreateSessionRequest(req CreateSessionRequest) error {
	if strings.TrimSpace(req.PackageCode) == "" || strings.TrimSpace(req.Version) == "" ||
		req.Seed == 0 || !sourceRefRe.MatchString(strings.TrimSpace(req.SourceRef)) {
		return apperr.ErrSimSessionInvalid
	}
	if _, ok := ids.Parse(req.OwnerAccountID); !ok {
		return apperr.ErrSimSessionInvalid
	}
	return nil
}

// validateNextAction 校验操作序列连续性和重复上报幂等性。
func validateNextAction(lastSeq int32, existing *ActionDTO, req ReportActionRequest) error {
	// 第一步基础字段必须完整,事件类型为空会导致回放无法复现。
	if req.Seq <= 0 || req.AtTick < 0 || strings.TrimSpace(req.EventType) == "" {
		return apperr.ErrSimActionInvalid
	}
	// 第二步同 seq 已存在时只接受完全相同内容,作为网络重试的幂等成功。
	if existing != nil {
		if existing.Seq == req.Seq && sameAction(*existing, req) {
			return nil
		}
		return apperr.ErrSimActionInvalid
	}
	// 第三步新操作必须紧跟 lastSeq,不接受跳号写入破坏回放顺序。
	if req.Seq != lastSeq+1 {
		return apperr.ErrSimActionInvalid
	}
	return nil
}

// validateCheckpointRequest 校验检查点上报,answer 可为空对象但 checkpoint_id 必须稳定。
func validateCheckpointRequest(req ReportCheckpointRequest) error {
	if strings.TrimSpace(req.CheckpointID) == "" {
		return apperr.ErrSimCheckpointInvalid
	}
	return nil
}

// validateShareCode 校验公开分享码格式,避免非法输入进入全局索引查询路径。
func validateShareCode(code string) error {
	if !shareCodeRe.MatchString(strings.TrimSpace(code)) {
		return apperr.ErrSimShareInvalid
	}
	return nil
}

// authorizeSessionOwner 校验用户侧会话操作只能由会话 owner 执行。
func authorizeSessionOwner(id tenant.Identity, ownerID int64) error {
	if id.AccountID <= 0 || ownerID <= 0 || id.AccountID != ownerID {
		return apperr.ErrSimAccessDenied
	}
	return nil
}

// validateSimSourceRefAccess 校验服务签名绑定的 source_ref 与会话归属一致。
func validateSimSourceRefAccess(ctx context.Context, sourceRef string) error {
	signedSourceRef, ok := auth.ServiceSourceRefFromContext(ctx)
	if !ok {
		return nil
	}
	if signedSourceRef != sourceRef {
		return apperr.ErrSimAccessDenied
	}
	return nil
}

// parseCompute 把 API 字符串运行位置转为数据库枚举。
func parseCompute(v string) (int16, error) {
	switch strings.TrimSpace(v) {
	case "", "frontend":
		return ComputeFrontend, nil
	case "backend":
		return ComputeBackend, nil
	default:
		return 0, apperr.ErrSimPackageInvalid
	}
}

// computeString 把数据库运行位置转为 API 字符串。
func computeString(v int16) string {
	if v == ComputeBackend {
		return "backend"
	}
	return "frontend"
}

// sameAction 对比重复上报操作是否完全一致。
func sameAction(existing ActionDTO, req ReportActionRequest) bool {
	if existing.Seq != req.Seq || existing.AtTick != req.AtTick || existing.EventType != req.EventType {
		return false
	}
	return jsonx.Equal(existing.Payload, req.Payload)
}
