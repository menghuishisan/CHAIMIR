// M4 校验测试:覆盖仿真包、会话与操作序列的边界规则。
package sim

import (
	"context"
	"os"
	"strings"
	"testing"

	"chaimir/internal/platform/auth"
	"chaimir/internal/platform/tenant"
	"chaimir/pkg/apperr"
)

// TestValidatePackageRequestRequiresStableNamespaceAndSemver 确认仿真包提交必须具备命名空间和 semver。
func TestValidatePackageRequestRequiresStableNamespaceAndSemver(t *testing.T) {
	valid := SubmitPackageRequest{
		Code: "teacher_12__pow-mining", Version: "1.2.3", Name: "PoW mining",
		Category: "consensus", Compute: "frontend", BundleKey: "sim/pkg/pow.tgz",
		BundleHash: "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef",
		AuthorType: AuthorTypeTeacher, AuthorID: "12",
	}
	if err := validateSubmitPackageRequest(valid); err != nil {
		t.Fatalf("expected valid package request, got %v", err)
	}

	cases := []SubmitPackageRequest{
		{Code: "pow", Version: "1.0.0", Name: "PoW", Category: "consensus", Compute: "frontend", BundleKey: "sim/pkg/pow.tgz", BundleHash: valid.BundleHash, AuthorType: AuthorTypeTeacher, AuthorID: "12"},
		{Code: "teacher_12__pow", Version: "v1", Name: "PoW", Category: "consensus", Compute: "frontend", BundleKey: "sim/pkg/pow.tgz", BundleHash: valid.BundleHash, AuthorType: AuthorTypeTeacher, AuthorID: "12"},
		{Code: "teacher_12__pow", Version: "1.0.0", Name: "PoW", Category: "consensus", Compute: "backend", BundleKey: "sim/pkg/pow.tgz", BundleHash: valid.BundleHash, AuthorType: AuthorTypeTeacher, AuthorID: "12"},
	}
	for _, req := range cases {
		if err := validateSubmitPackageRequest(req); err == nil {
			t.Fatalf("expected invalid package request for %+v", req)
		}
	}
}

// TestValidatePackageRequestRejectsInvalidAuthorID 确认作者 ID 不能解析失败后静默落为 0。
func TestValidatePackageRequestRejectsInvalidAuthorID(t *testing.T) {
	req := SubmitPackageRequest{
		Code: "teacher_12__pow-mining", Version: "1.2.3", Name: "PoW mining",
		Category: "consensus", Compute: "frontend", BundleKey: "sim/pkg/pow.tgz",
		BundleHash: "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef",
		AuthorType: AuthorTypeTeacher, AuthorID: "not-an-id",
	}
	if err := validateSubmitPackageRequest(req); err == nil {
		t.Fatalf("expected invalid author id to be rejected")
	}
}

// TestValidatePackageRequestRejectsTeacherUsingBuiltinNamespace 确认教师包不能覆盖平台内置命名空间。
func TestValidatePackageRequestRejectsTeacherUsingBuiltinNamespace(t *testing.T) {
	req := SubmitPackageRequest{
		Code: "builtin__pow-mining", Version: "1.2.3", Name: "PoW mining",
		Category: "consensus", Compute: "frontend", BundleKey: "sim/pkg/pow.tgz",
		BundleHash: "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef",
		AuthorType: AuthorTypeTeacher, AuthorID: "12",
	}
	if err := validateSubmitPackageRequest(req); err == nil {
		t.Fatalf("expected teacher package using builtin namespace to be rejected")
	}
}

// TestJSONFormRejectsInvalidJSON 确认表单 JSON 字段非法时不会被静默当成空对象。
func TestJSONFormRejectsInvalidJSON(t *testing.T) {
	_, err := jsonForm(`{"nodes":`)
	if err == nil {
		t.Fatalf("expected invalid form JSON to fail")
	}
	if ae, ok := apperr.As(err); !ok || ae.Code != apperr.ErrSimPackageInvalid.Code {
		t.Fatalf("expected sim package invalid error, got %v", err)
	}
}

// TestValidateActionSequenceAllowsIdempotentReplay 确认操作上报必须连续,同序号同内容可幂等重放。
func TestValidateActionSequenceAllowsIdempotentReplay(t *testing.T) {
	action := ReportActionRequest{Seq: 3, AtTick: 9, EventType: "launch-51", Payload: map[string]any{"blocks": float64(6)}}
	existing := ActionDTO{Seq: 3, AtTick: 9, EventType: "launch-51", Payload: map[string]any{"blocks": float64(6)}}
	if err := validateNextAction(2, nil, action); err != nil {
		t.Fatalf("expected next action to be accepted, got %v", err)
	}
	if err := validateNextAction(3, &existing, action); err != nil {
		t.Fatalf("expected identical retry to be accepted, got %v", err)
	}
	action.Payload = map[string]any{"blocks": float64(7)}
	if err := validateNextAction(3, &existing, action); err == nil {
		t.Fatalf("expected conflicting retry to be rejected")
	}
	action.Seq = 5
	if err := validateNextAction(3, nil, action); err == nil {
		t.Fatalf("expected sequence gap to be rejected")
	}
}

// TestAuthorizeSessionOwnerRejectsOtherAccount 确认用户侧会话接口不能只依赖租户 RLS。
func TestAuthorizeSessionOwnerRejectsOtherAccount(t *testing.T) {
	owner := tenant.Identity{TenantID: 1001, AccountID: 2001}
	if err := authorizeSessionOwner(owner, 2001); err != nil {
		t.Fatalf("session owner should access own session: %v", err)
	}

	other := tenant.Identity{TenantID: 1001, AccountID: 2002}
	err := authorizeSessionOwner(other, 2001)
	if ae, ok := apperr.As(err); !ok || ae.Code != apperr.ErrSimAccessDenied.Code {
		t.Fatalf("other account must be forbidden, got %v", err)
	}
}

// TestValidateSimSourceRefAccessRejectsSignedMismatch 确认内部服务签名绑定的 source_ref 不能访问其他来源会话。
func TestValidateSimSourceRefAccessRejectsSignedMismatch(t *testing.T) {
	if err := validateSimSourceRefAccess(context.Background(), "experiment:2026:instance:55"); err != nil {
		t.Fatalf("no signed source_ref should be left to caller context, got %v", err)
	}

	ctx := auth.WithServiceSourceRef(context.Background(), "experiment:2026:instance:55")
	if err := validateSimSourceRefAccess(ctx, "experiment:2026:instance:55"); err != nil {
		t.Fatalf("matching signed source_ref should pass, got %v", err)
	}

	err := validateSimSourceRefAccess(ctx, "experiment:2026:instance:56")
	if ae, ok := apperr.As(err); !ok || ae.Code != apperr.ErrSimAccessDenied.Code {
		t.Fatalf("mismatched signed source_ref must return sim access denied, got %v", err)
	}
}

// TestArchivePathsPublishSessionEndedEvent 确认归档和来源回收都通过事件总线通知上层。
func TestArchivePathsPublishSessionEndedEvent(t *testing.T) {
	data, err := os.ReadFile("service.go")
	if err != nil {
		t.Fatalf("read service.go: %v", err)
	}
	body := string(data)
	for _, tc := range []struct {
		name  string
		start string
		end   string
	}{
		{name: "recycle by source ref", start: "func (s *Service) RecycleBySourceRef(", end: "// ArchiveSession"},
		{name: "archive single session", start: "func (s *Service) ArchiveSession(", end: "// ShareSession"},
	} {
		start := strings.Index(body, tc.start)
		end := strings.Index(body, tc.end)
		if start < 0 || end < start {
			t.Fatalf("%s function block not found", tc.name)
		}
		block := body[start:end]
		if !strings.Contains(block, "publishSessionEnded") {
			t.Fatalf("%s must publish sim.session.ended event after archive", tc.name)
		}
	}
}

// TestReplayInTenantValidatesSignedSourceRef 确认内部回放读取也绑定服务签名来源。
func TestReplayInTenantValidatesSignedSourceRef(t *testing.T) {
	data, err := os.ReadFile("service.go")
	if err != nil {
		t.Fatalf("read service.go: %v", err)
	}
	body := string(data)
	start := strings.Index(body, "func (s *Service) replayInTenant(")
	end := strings.Index(body, "// loadBackendSession")
	if start < 0 || end < start {
		t.Fatalf("replayInTenant function block not found")
	}
	block := body[start:end]
	if !strings.Contains(block, "repo.replayInTenant") {
		t.Fatalf("replayInTenant service must delegate signed source_ref checked loading to repo")
	}
	repoData, err := os.ReadFile("repo.go")
	if err != nil {
		t.Fatalf("read repo.go: %v", err)
	}
	if !strings.Contains(string(repoData), "validateSimSourceRefAccess(ctx, row.SourceRef)") {
		t.Fatalf("repo replay loading must validate signed source_ref before returning replay data")
	}
}

// TestValidatePackageAuthorAccessRejectsOtherTeacher 确认教师只能维护自己的扩展包。
func TestValidatePackageAuthorAccessRejectsOtherTeacher(t *testing.T) {
	pkg := packageAuthorScope{
		AuthorType: AuthorTypeTeacher,
		AuthorID:   2001,
	}
	if err := validatePackageAuthorAccess(tenant.Identity{TenantID: 10, AccountID: 2001}, pkg); err != nil {
		t.Fatalf("author should update own package: %v", err)
	}
	if err := validatePackageAuthorAccess(tenant.Identity{IsPlatform: true, AccountID: 9001}, pkg); err != nil {
		t.Fatalf("platform admin should update package: %v", err)
	}
	err := validatePackageAuthorAccess(tenant.Identity{TenantID: 10, AccountID: 2002}, pkg)
	if ae, ok := apperr.As(err); !ok || ae.Code != apperr.ErrSimAccessDenied.Code {
		t.Fatalf("other teacher must be denied, got %v", err)
	}
}

// TestValidatePackageSubmitterAccessRejectsTeacherBuiltin 确认教师不能冒充平台内置包提交者。
func TestValidatePackageSubmitterAccessRejectsTeacherBuiltin(t *testing.T) {
	teacher := tenant.Identity{TenantID: 10, AccountID: 2001}
	if err := validatePackageSubmitterAccess(teacher, SubmitPackageRequest{AuthorType: AuthorTypeBuiltin}); err == nil {
		t.Fatalf("teacher must not submit builtin package")
	}
	if err := validatePackageSubmitterAccess(teacher, SubmitPackageRequest{AuthorType: AuthorTypeTeacher, AuthorID: "2001"}); err != nil {
		t.Fatalf("teacher should submit own package: %v", err)
	}
	if err := validatePackageSubmitterAccess(tenant.Identity{IsPlatform: true, AccountID: 9001}, SubmitPackageRequest{AuthorType: AuthorTypeBuiltin}); err != nil {
		t.Fatalf("platform admin should submit builtin package: %v", err)
	}
}

// TestValidatePackageListStatusAccessRestrictsNonPlatform 确认普通用户不能枚举草稿、审核中或退回包。
func TestValidatePackageListStatusAccessRestrictsNonPlatform(t *testing.T) {
	teacher := tenant.Identity{TenantID: 10, AccountID: 2001}
	if err := validatePackageListStatusAccess(teacher, PackageStatusPublished); err != nil {
		t.Fatalf("published package list should be visible to users: %v", err)
	}
	err := validatePackageListStatusAccess(teacher, PackageStatusReviewing)
	if ae, ok := apperr.As(err); !ok || ae.Code != apperr.ErrSimAccessDenied.Code {
		t.Fatalf("non-platform listing reviewing packages must be denied, got %v", err)
	}
	if err := validatePackageListStatusAccess(tenant.Identity{IsPlatform: true, AccountID: 9001}, PackageStatusReviewing); err != nil {
		t.Fatalf("platform admin should query reviewing packages: %v", err)
	}
}

// TestSubmitAndUpdateValidateMetadataBeforeUpload 确认非法元数据不会先写入对象存储。
func TestSubmitAndUpdateValidateMetadataBeforeUpload(t *testing.T) {
	data, err := os.ReadFile("api.go")
	if err != nil {
		t.Fatalf("read api.go: %v", err)
	}
	body := string(data)
	for _, tc := range []struct {
		name  string
		start string
		end   string
	}{
		{name: "submit", start: "func (a *API) submitPackage(", end: "// readAndStoreBundle"},
		{name: "update", start: "func (a *API) updatePackage(", end: "// listReviews"},
	} {
		start := strings.Index(body, tc.start)
		end := strings.Index(body, tc.end)
		if start < 0 || end < start {
			t.Fatalf("%s package handler block not found", tc.name)
		}
		block := body[start:end]
		validateIdx := strings.Index(block, "validatePackageUploadMetadata")
		uploadIdx := strings.Index(block, "readAndStoreBundle")
		if validateIdx < 0 || uploadIdx < 0 || validateIdx > uploadIdx {
			t.Fatalf("%s handler must validate metadata before reading/storing bundle", tc.name)
		}
	}
}

// TestUpdatePackageCreatesFreshPendingReview 确认退回包更新后重新进入待审队列。
func TestUpdatePackageCreatesFreshPendingReview(t *testing.T) {
	data, err := os.ReadFile("service.go")
	if err != nil {
		t.Fatalf("read service.go: %v", err)
	}
	body := string(data)
	start := strings.Index(body, "func (s *Service) UpdateUploadedPackage(")
	end := strings.Index(body, "// GetPackagePreview")
	if start < 0 || end < start {
		t.Fatalf("UpdatePackage function block not found")
	}
	block := body[start:end]
	if !strings.Contains(block, "updatePackageDraftWithReview") {
		t.Fatalf("UpdatePackage must delegate draft update and fresh review creation to repo")
	}
	repoData, err := os.ReadFile("repo.go")
	if err != nil {
		t.Fatalf("read repo.go: %v", err)
	}
	if !strings.Contains(string(repoData), "CreateSimPackageReview") {
		t.Fatalf("repo update must create a fresh pending review after updating draft/rejected package")
	}
}

// TestUpdatePackageRouteScansUploadedBundle 确认更新包不能直接信任客户端传入 bundle_key/hash。
func TestUpdatePackageRouteScansUploadedBundle(t *testing.T) {
	data, err := os.ReadFile("api.go")
	if err != nil {
		t.Fatalf("read api.go: %v", err)
	}
	body := string(data)
	start := strings.Index(body, "func (a *API) updatePackage(")
	end := strings.Index(body, "// listReviews")
	if start < 0 || end < start {
		t.Fatalf("updatePackage function block not found")
	}
	block := body[start:end]
	for _, required := range []string{
		"readAndStoreBundle",
		"UpdateUploadedPackage",
	} {
		if !strings.Contains(block, required) {
			t.Fatalf("updatePackage must use uploaded bundle scanning path, missing %s", required)
		}
	}
	if strings.Contains(block, "ShouldBindJSON") {
		t.Fatalf("updatePackage must not accept direct JSON bundle_key/bundle_hash updates")
	}
	helperStart := strings.Index(body, "func (a *API) readAndStoreBundle(")
	helperEnd := strings.Index(body, "// validatePackageUploadMetadata")
	if helperStart < 0 || helperEnd < helperStart {
		t.Fatalf("readAndStoreBundle helper block not found")
	}
	helper := body[helperStart:helperEnd]
	for _, required := range []string{`FormFile("bundle")`, "StoreUploadedBundle"} {
		if !strings.Contains(helper, required) {
			t.Fatalf("shared bundle upload helper must read input and delegate scan/storage to service, missing %s", required)
		}
	}
	for _, forbidden := range []string{"scanBundleWithLimits", "bundleHash(data)", "store.Put"} {
		if strings.Contains(helper, forbidden) {
			t.Fatalf("API helper must not perform bundle scan/storage directly, found %s", forbidden)
		}
	}
	serviceData, err := os.ReadFile("service_bundle.go")
	if err != nil {
		t.Fatalf("read service_bundle.go: %v", err)
	}
	serviceBody := string(serviceData)
	storeStart := strings.Index(serviceBody, "func (s *Service) StoreUploadedBundle(")
	if storeStart < 0 {
		t.Fatalf("service StoreUploadedBundle block not found")
	}
	storeBlock := serviceBody[storeStart:]
	for _, required := range []string{"scanBundleWithLimits", "bundleHash(data)", "store.Put"} {
		if !strings.Contains(storeBlock, required) {
			t.Fatalf("service upload path must enforce backend scan/storage, missing %s", required)
		}
	}
}

// TestMergeValidationReportPreservesBackendScanAuthority 确认受控预览报告不能覆盖上传扫描生成的权威字段。
func TestMergeValidationReportPreservesBackendScanAuthority(t *testing.T) {
	current := map[string]any{
		"metadata_validation": "passed",
		"static_scan":         "failed",
		"bundle_hash":         "backend-hash",
		"file_count":          float64(3),
	}
	incoming := map[string]any{
		"static_scan":       "passed",
		"bundle_hash":       "client-hash",
		"determinism_check": "passed",
		"worker_preview":    "passed",
	}

	_, err := mergeValidationReport(current, incoming)
	if ae, ok := apperr.As(err); !ok || ae.Code != apperr.ErrSimPackageValidationFail.Code {
		t.Fatalf("validation report must reject attempts to override backend scan fields, got %v", err)
	}
}

// TestMergeValidationReportAddsOnlyDynamicChecks 确认受控预览流程只能补充确定性和 Worker 预览结果。
func TestMergeValidationReportAddsOnlyDynamicChecks(t *testing.T) {
	current := map[string]any{
		"metadata_validation": "passed",
		"static_scan":         "passed",
		"bundle_hash":         "backend-hash",
	}
	merged, err := mergeValidationReport(current, map[string]any{
		"determinism_check": "passed",
		"worker_preview":    "passed",
	})
	if err != nil {
		t.Fatalf("expected dynamic validation report to merge, got %v", err)
	}
	if merged["static_scan"] != "passed" || merged["bundle_hash"] != "backend-hash" {
		t.Fatalf("backend scan fields must be preserved, got %+v", merged)
	}
	if merged["determinism_check"] != "passed" || merged["worker_preview"] != "passed" {
		t.Fatalf("dynamic validation fields missing, got %+v", merged)
	}
}

// TestPreviewReportRequiresWorkerPreview 确认上架门禁同时要求静态扫描、确定性校验和沙箱化 Worker 预览通过。
func TestPreviewReportRequiresWorkerPreview(t *testing.T) {
	if previewReportPassed(map[string]any{
		"static_scan":         "passed",
		"determinism_check":   "passed",
		"worker_preview":      "failed",
		"metadata_validation": "passed",
	}) {
		t.Fatalf("review must not pass without worker preview")
	}
	if !previewReportPassed(map[string]any{
		"static_scan":         "passed",
		"determinism_check":   "passed",
		"worker_preview":      "passed",
		"metadata_validation": "passed",
	}) {
		t.Fatalf("review should pass when all validation gates passed")
	}
}

// TestServiceDoesNotExposeUnscannedSubmitPackage 确认生产服务层不保留绕过上传扫描的包提交入口。
func TestServiceDoesNotExposeUnscannedSubmitPackage(t *testing.T) {
	data, err := os.ReadFile("service.go")
	if err != nil {
		t.Fatalf("read service.go: %v", err)
	}
	if strings.Contains(string(data), "func (s *Service) SubmitPackage(") {
		t.Fatalf("Service must expose only SubmitUploadedPackage so all package submissions pass backend scanning")
	}
}

// TestSimProductionCodeDoesNotUseGenericInternalError 确认 M4 生产代码不把真实业务错误折叠为通用 11500。
func TestSimProductionCodeDoesNotUseGenericInternalError(t *testing.T) {
	for _, filename := range []string{"service.go", "audit.go"} {
		data, err := os.ReadFile(filename)
		if err != nil {
			t.Fatalf("read %s: %v", filename, err)
		}
		if strings.Contains(string(data), "ErrInternal") {
			t.Fatalf("%s must use M4-specific errors instead of ErrInternal", filename)
		}
	}
}
