// M1 错误语义测试:防止账号和导入流程回退通用错误码。
package identity

import (
	"os"
	"strings"
	"testing"

	"chaimir/pkg/apperr"
)

// TestIdentityAccountAndImportUseDedicatedErrors 防止账号冲突和导入预览未命中复用通用 110xx 错误。
func TestIdentityAccountAndImportUseDedicatedErrors(t *testing.T) {
	for _, path := range []string{"service_account.go", "service_import.go"} {
		src, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read %s: %v", path, err)
		}
		text := string(src)
		for _, forbidden := range []string{"ErrConflict", "ErrNotFound"} {
			if strings.Contains(text, forbidden) {
				t.Fatalf("%s must use dedicated identity errors instead of %s", path, forbidden)
			}
		}
	}
}

// TestIdentityPagedEndpointsUseDatabaseTotals 防止分页接口把当前页长度当作总数返回。
func TestIdentityPagedEndpointsUseDatabaseTotals(t *testing.T) {
	for _, tt := range []struct {
		path     string
		required string
	}{
		{path: "service_import.go", required: "CountImportBatches"},
		{path: "service_platform.go", required: "CountTenantApplications"},
		{path: "contract_impl.go", required: "CountAuditLogs"},
	} {
		src, err := os.ReadFile(tt.path)
		if err != nil {
			t.Fatalf("read %s: %v", tt.path, err)
		}
		if !strings.Contains(string(src), tt.required) {
			t.Fatalf("%s must use %s for page total", tt.path, tt.required)
		}
	}
	api, err := os.ReadFile("api_account.go")
	if err != nil {
		t.Fatalf("read api_account.go: %v", err)
	}
	if strings.Contains(string(api), "int64(len(rows))") {
		t.Fatalf("api_account.go must not return current page length as total")
	}
}

// TestIdentityAccountMutationsCheckTargetExists 防止账号写操作对不存在账号静默成功并写审计。
func TestIdentityAccountMutationsCheckTargetExists(t *testing.T) {
	src, err := os.ReadFile("service_account.go")
	if err != nil {
		t.Fatalf("read service_account.go: %v", err)
	}
	text := string(src)
	for _, name := range []string{"UpdateAccount", "ForceLogout", "RevokeAdmin"} {
		body := functionBody(t, text, name)
		if !strings.Contains(body, "GetAccountByID") || !strings.Contains(body, "ErrAccountNotFound") {
			t.Fatalf("%s must check target account existence and return ErrAccountNotFound", name)
		}
	}
}

// TestIdentityValidationUsesScenarioSpecificCodes 防止不同业务校验共用同一个泛化错误码。
func TestIdentityValidationUsesScenarioSpecificCodes(t *testing.T) {
	if err := validateTenantStatus(99); err != apperr.ErrTenantStatusInvalid {
		t.Fatalf("invalid tenant status must use ErrTenantStatusInvalid, got %v", err)
	}
	if err := validateAuthMode(99); err != apperr.ErrTenantAuthModeInvalid {
		t.Fatalf("invalid auth mode must use ErrTenantAuthModeInvalid, got %v", err)
	}
	if err := validateSchoolType(99); err != apperr.ErrSchoolTypeInvalid {
		t.Fatalf("invalid school type must use ErrSchoolTypeInvalid, got %v", err)
	}
	if err := validateApplicationStatus(99); err != apperr.ErrApplicationStatusInvalid {
		t.Fatalf("invalid application status must use ErrApplicationStatusInvalid, got %v", err)
	}
}

// TestIdentityAmbiguousResetUsesDedicatedCode 确认一号多校找回密码不复用账号参数错误。
func TestIdentityAmbiguousResetUsesDedicatedCode(t *testing.T) {
	src, err := os.ReadFile("service_auth.go")
	if err != nil {
		t.Fatalf("read service_auth.go: %v", err)
	}
	body := functionBody(t, string(src), "selectResetPasswordTarget")
	if !strings.Contains(body, "ErrResetPasswordTenantAmbiguous") {
		t.Fatalf("selectResetPasswordTarget must use ErrResetPasswordTenantAmbiguous")
	}
}

// TestIdentityGlobalSmsVerificationRequiresPrivilegedConnection 防止找回验证码校验缺少特权连接前置检查。
func TestIdentityGlobalSmsVerificationRequiresPrivilegedConnection(t *testing.T) {
	src, err := os.ReadFile("service_sms.go")
	if err != nil {
		t.Fatalf("read service_sms.go: %v", err)
	}
	body := functionBody(t, string(src), "verifySmsCode")
	if !strings.Contains(body, "!s.repo.hasPrivileged()") {
		t.Fatalf("verifySmsCode must check privileged connection before NULL-tenant verification")
	}
}

// TestIdentityJSONBSerializationUsesScenarioCodes 确认 JSONB 边界使用所属场景专属错误码。
func TestIdentityJSONBSerializationUsesScenarioCodes(t *testing.T) {
	checks := []struct {
		path      string
		forbidden string
		required  string
	}{
		{path: "service_platform.go", forbidden: "jsonx.ObjectBytes(req.FeatureFlags, apperr.ErrTenantInvalid)", required: "ErrTenantFeatureFlagsInvalid"},
		{path: "service_import.go", forbidden: "apperr.ErrImportInvalid", required: "ErrImportRowsInvalid"},
		{path: "service_org.go", forbidden: "apperr.ErrImportInvalid", required: "ErrOrgImportRowsInvalid"},
	}
	for _, tt := range checks {
		src, err := os.ReadFile(tt.path)
		if err != nil {
			t.Fatalf("read %s: %v", tt.path, err)
		}
		text := string(src)
		if strings.Contains(text, tt.forbidden) && !strings.Contains(text, tt.required) {
			t.Fatalf("%s must use %s instead of shared %s at JSONB boundary", tt.path, tt.required, tt.forbidden)
		}
	}
}

// TestIdentityImportPreviewPreservesJSONBScenarioCodes 防止导入预览 JSONB 错误被改写成内部错误。
func TestIdentityImportPreviewPreservesJSONBScenarioCodes(t *testing.T) {
	src, err := os.ReadFile("service_import.go")
	if err != nil {
		t.Fatalf("read service_import.go: %v", err)
	}
	body := functionBody(t, string(src), "CreateImportPreview")
	for _, required := range []string{
		"rowsJSON, err := marshalImportRows(req.Rows)\n\tif err != nil {\n\t\treturn nil, toAppErr(err)",
		"resultJSON, err := marshalImportPreviewResult(result)\n\tif err != nil {\n\t\treturn nil, toAppErr(err)",
	} {
		if !strings.Contains(body, required) {
			t.Fatalf("CreateImportPreview must preserve JSONB scenario errors instead of wrapping them as internal errors")
		}
	}
}

// TestIdentityNoSharedGenericCodesRemain 防止 M1 生产代码继续复用过宽的账号/租户/导入泛化码。
func TestIdentityNoSharedGenericCodesRemain(t *testing.T) {
	for _, path := range []string{
		"service_account.go",
		"service_platform.go",
		"api_org.go",
		"service_import.go",
		"import_template.go",
	} {
		src, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read %s: %v", path, err)
		}
		for _, forbidden := range []string{"ErrAccountInvalid", "ErrTenantInvalid", "ErrImportInvalid"} {
			if strings.Contains(string(src), forbidden) {
				t.Fatalf("%s must not use shared generic identity code %s", path, forbidden)
			}
		}
	}
}

// TestIdentityOrgMutationsCheckTargetExists 防止组织更新/删除对不存在对象静默成功并写审计。
func TestIdentityOrgMutationsCheckTargetExists(t *testing.T) {
	src, err := os.ReadFile("service_org.go")
	if err != nil {
		t.Fatalf("read service_org.go: %v", err)
	}
	text := string(src)
	for _, tt := range []struct {
		name     string
		getter   string
		notFound string
	}{
		{name: "UpdateDepartment", getter: "GetDepartmentByID", notFound: "ErrDepartmentNotFound"},
		{name: "DeleteDepartment", getter: "GetDepartmentByID", notFound: "ErrDepartmentNotFound"},
		{name: "DeleteMajor", getter: "GetMajorByID", notFound: "ErrMajorNotFound"},
		{name: "DeleteClass", getter: "GetClassByID", notFound: "ErrClassNotFound"},
	} {
		body := functionBody(t, text, tt.name)
		if !strings.Contains(body, tt.getter) || !strings.Contains(body, tt.notFound) {
			t.Fatalf("%s must check %s and return %s before mutating", tt.name, tt.getter, tt.notFound)
		}
	}
}

// TestIdentityApproveApplicationUsesSingleTransaction 防止入驻审核拆成平台与租户两段提交。
func TestIdentityApproveApplicationUsesSingleTransaction(t *testing.T) {
	src, err := os.ReadFile("service_platform.go")
	if err != nil {
		t.Fatalf("read service_platform.go: %v", err)
	}
	body := functionBody(t, string(src), "ApproveApplication")
	if strings.Contains(body, "s.repo.inApp(ctx") && strings.Contains(body, "s.repo.inTenantID(ctx") {
		t.Fatalf("ApproveApplication must not split tenant creation and first admin creation into two transactions")
	}
	if !strings.Contains(body, "inAppTenantID") {
		t.Fatalf("ApproveApplication must use one mixed platform/tenant transaction")
	}
}

// TestIdentityTenantMutationsCheckTargetExists 防止租户更新/配置修改未命中时返回内部错误。
func TestIdentityTenantMutationsCheckTargetExists(t *testing.T) {
	src, err := os.ReadFile("service_platform.go")
	if err != nil {
		t.Fatalf("read service_platform.go: %v", err)
	}
	text := string(src)
	for _, name := range []string{"UpdateTenant", "UpdateTenantConfig"} {
		body := functionBody(t, text, name)
		if !strings.Contains(body, "GetTenantByID") || !strings.Contains(body, "ErrTenantNotFound") {
			t.Fatalf("%s must check tenant exists and return ErrTenantNotFound", name)
		}
	}
}

// TestIdentitySSOConfigUsesScenarioSpecificCodes 防止 SSO 配置保存把不同错误都塞进 ErrSsoConfigInvalid。
func TestIdentitySSOConfigUsesScenarioSpecificCodes(t *testing.T) {
	if err := validateSsoConfigForStorage(99, map[string]any{}); err != apperr.ErrSsoTypeInvalid {
		t.Fatalf("invalid sso type must use ErrSsoTypeInvalid, got %v", err)
	}
	if err := validateSsoConfigForStorage(SsoTypeCAS, map[string]any{"server_url": "not-a-url"}); err != apperr.ErrSsoCASURLInvalid {
		t.Fatalf("invalid cas url must use ErrSsoCASURLInvalid, got %v", err)
	}
	if err := validateSsoConfigForStorage(SsoTypeLDAP, map[string]any{"url": "ldaps://ldap.example.edu"}); err != apperr.ErrSsoLDAPConfigInvalid {
		t.Fatalf("incomplete ldap config must use ErrSsoLDAPConfigInvalid, got %v", err)
	}
	src, err := os.ReadFile("service_platform.go")
	if err != nil {
		t.Fatalf("read service_platform.go: %v", err)
	}
	body := functionBody(t, string(src), "UpsertSsoConfig")
	for _, forbidden := range []string{"ErrSsoConfigInvalid"} {
		if strings.Contains(body, forbidden) {
			t.Fatalf("UpsertSsoConfig must not use shared %s", forbidden)
		}
	}
}

// TestIdentityOrgValidationUsesScenarioSpecificCodes 防止组织 ID 和批量输入复用 ErrOrgInvalid。
func TestIdentityOrgValidationUsesScenarioSpecificCodes(t *testing.T) {
	src, err := os.ReadFile("service_org.go")
	if err != nil {
		t.Fatalf("read service_org.go: %v", err)
	}
	text := string(src)
	for _, forbidden := range []string{"ErrOrgInvalid"} {
		if strings.Contains(text, forbidden) {
			t.Fatalf("service_org.go must not use shared %s", forbidden)
		}
	}
}

// TestIdentityAPIBindingUsesScenarioSpecificCodes 防止 HTTP JSON 绑定失败共用泛化错误码。
func TestIdentityAPIBindingUsesScenarioSpecificCodes(t *testing.T) {
	for _, tt := range []struct {
		path      string
		forbidden string
	}{
		{path: "api_org.go", forbidden: "ErrOrgInvalid"},
		{path: "api_platform.go", forbidden: "ErrSsoConfigInvalid"},
	} {
		src, err := os.ReadFile(tt.path)
		if err != nil {
			t.Fatalf("read %s: %v", tt.path, err)
		}
		if strings.Contains(string(src), tt.forbidden) {
			t.Fatalf("%s must not use shared binding code %s", tt.path, tt.forbidden)
		}
	}
}

// TestIdentityOrgRoutesDoNotExposeDuplicateSingleClassArchivePromote 防止班级归档/升级同时存在单条和批量两套路由。
func TestIdentityOrgRoutesDoNotExposeDuplicateSingleClassArchivePromote(t *testing.T) {
	src, err := os.ReadFile("api.go")
	if err != nil {
		t.Fatalf("read api.go: %v", err)
	}
	text := string(src)
	for _, forbidden := range []string{`/classes/:id/archive`, `/classes/:id/promote`} {
		if strings.Contains(text, forbidden) {
			t.Fatalf("M1 must expose class archive/promote through batch routes only, found %s", forbidden)
		}
	}
}

// TestIdentityBatchArchiveAccountsUsesEnrollmentYear 防止按学年归档账号退化为按账号 ID 批量改状态。
func TestIdentityBatchArchiveAccountsUsesEnrollmentYear(t *testing.T) {
	api, err := os.ReadFile("api_account.go")
	if err != nil {
		t.Fatalf("read api_account.go: %v", err)
	}
	body := methodBody(t, string(api), "API", "batchArchiveAccounts")
	if strings.Contains(body, "batchSetAccountStatus") || !strings.Contains(body, "BatchArchiveAccounts") {
		t.Fatalf("batchArchiveAccounts must use enrollment-year archive flow, not generic account_ids status flow")
	}

	dto, err := os.ReadFile("dto.go")
	if err != nil {
		t.Fatalf("read dto.go: %v", err)
	}
	if !strings.Contains(string(dto), "type BatchArchiveAccountsRequest struct") || !strings.Contains(string(dto), "EnrollmentYear int16") {
		t.Fatalf("dto.go must define BatchArchiveAccountsRequest with enrollment_year")
	}

	query, err := os.ReadFile("../../../db/queries/identity.sql")
	if err != nil {
		t.Fatalf("read identity.sql: %v", err)
	}
	if !strings.Contains(string(query), "ArchiveStudentAccountsByEnrollmentYear") {
		t.Fatalf("identity.sql must archive student accounts by enrollment_year")
	}
}

// TestIdentityRoleCodesUseContracts 防止 M1 角色字符串码绕过 contracts。
func TestIdentityRoleCodesUseContracts(t *testing.T) {
	src, err := os.ReadFile("enum.go")
	if err != nil {
		t.Fatalf("read enum.go: %v", err)
	}
	if strings.Contains(string(src), `"platform_admin"`) {
		t.Fatalf("identity role string codes must come from contracts")
	}
}

// TestIdentityPlatformLoginUsesPlatformScopedToken 防止平台管理员登录复用租户账号会话。
func TestIdentityPlatformLoginUsesPlatformScopedToken(t *testing.T) {
	src, err := os.ReadFile("service_platform_login.go")
	if err != nil {
		t.Fatalf("read service_platform_login.go: %v", err)
	}
	text := string(src)
	for _, required := range []string{
		"LoginPlatform",
		"IssueAccess(0, admin.ID, sessionID, true)",
		"CreatePlatformAuthSession",
		"buildPlatformAuditEntry",
		"CreateAuditLog",
		"AuditActionAuthLogin",
	} {
		if !strings.Contains(text, required) {
			t.Fatalf("platform login must use platform-scoped token/session, missing %s", required)
		}
	}
	if strings.Contains(text, "CreateAuthSession") {
		t.Fatalf("platform login must not write tenant auth_session")
	}
}

// TestIdentityPlatformTokensHaveRefreshAndLogoutPaths 防止平台登录只签发 token 但无法刷新/登出。
func TestIdentityPlatformTokensHaveRefreshAndLogoutPaths(t *testing.T) {
	authSrc, err := os.ReadFile("service_auth.go")
	if err != nil {
		t.Fatalf("read service_auth.go: %v", err)
	}
	authText := string(authSrc)
	for _, tt := range []struct {
		name     string
		required []string
	}{
		{name: "Refresh", required: []string{"refreshPlatform(ctx"}},
		{name: "refreshPlatform", required: []string{"findPlatformSessionByTokenHash", "RevokeAllPlatformAdminSessions", "issuePlatformLogin"}},
		{name: "Logout", required: []string{"LogoutPlatform"}},
		{name: "LogoutPlatform", required: []string{"RevokePlatformAuthSession"}},
	} {
		body := functionBody(t, authText, tt.name)
		for _, required := range tt.required {
			if !strings.Contains(body, required) {
				t.Fatalf("%s must handle platform session lifecycle, missing %s", tt.name, required)
			}
		}
	}
}

// TestIdentityPlatformLoginDisabledOutsideSaaS 防止私有化部署暴露平台管理员认证面。
func TestIdentityPlatformLoginDisabledOutsideSaaS(t *testing.T) {
	src, err := os.ReadFile("service_platform_login.go")
	if err != nil {
		t.Fatalf("read service_platform_login.go: %v", err)
	}
	body := functionBody(t, string(src), "LoginPlatform")
	if !strings.Contains(body, "!s.cfg.PlatformEnabled") || !strings.Contains(body, "ErrForbidden") {
		t.Fatalf("LoginPlatform must reject platform login when platform layer is disabled")
	}
}

// TestIdentityProductionCodeAvoidsGlobalInternalErrors 防止 M1 业务失败继续复用全局内部错误码。
func TestIdentityProductionCodeAvoidsGlobalInternalErrors(t *testing.T) {
	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatalf("read identity dir: %v", err)
	}
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".go") || strings.HasSuffix(entry.Name(), "_test.go") {
			continue
		}
		src, err := os.ReadFile(entry.Name())
		if err != nil {
			t.Fatalf("read %s: %v", entry.Name(), err)
		}
		if strings.Contains(string(src), "ErrInternal.WithCause") {
			t.Fatalf("%s must use identity-specific error codes instead of ErrInternal.WithCause", entry.Name())
		}
	}
}

// TestIdentityProductionCodeAvoidsFallbackTerminology 防止生产代码保留兜底式命名。
func TestIdentityProductionCodeAvoidsFallbackTerminology(t *testing.T) {
	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatalf("read identity dir: %v", err)
	}
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".go") || strings.HasSuffix(entry.Name(), "_test.go") {
			continue
		}
		src, err := os.ReadFile(entry.Name())
		if err != nil {
			t.Fatalf("read %s: %v", entry.Name(), err)
		}
		for _, forbidden := range []string{"fallback", "compat", "shim", "兼容", "兜底"} {
			if strings.Contains(string(src), forbidden) {
				t.Fatalf("%s must not contain fallback terminology %q in production code", entry.Name(), forbidden)
			}
		}
	}
}

func functionBody(t *testing.T, src, name string) string {
	t.Helper()
	start := strings.Index(src, "func (s *Service) "+name)
	if start < 0 {
		start = strings.Index(src, "func "+name)
	}
	if start < 0 {
		t.Fatalf("function %s not found", name)
	}
	next := strings.Index(src[start+1:], "\n// ")
	if next < 0 {
		return src[start:]
	}
	return src[start : start+1+next]
}

func methodBody(t *testing.T, src, receiverType, name string) string {
	t.Helper()
	start := strings.Index(src, "func (")
	for start >= 0 {
		endLine := strings.Index(src[start:], "\n")
		if endLine < 0 {
			t.Fatalf("method %s not found", name)
		}
		signature := src[start : start+endLine]
		if strings.Contains(signature, "*"+receiverType+") "+name) || strings.Contains(signature, receiverType+") "+name) {
			next := strings.Index(src[start+1:], "\n// ")
			if next < 0 {
				return src[start:]
			}
			return src[start : start+1+next]
		}
		nextStart := strings.Index(src[start+1:], "func (")
		if nextStart < 0 {
			break
		}
		start = start + 1 + nextStart
	}
	t.Fatalf("method %s not found", name)
	return ""
}
