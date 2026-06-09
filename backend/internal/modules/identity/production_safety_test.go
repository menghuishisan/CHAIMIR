// identity production_safety_test 文件守护生产安全与文档口径,避免关键流程回退。
package identity

import (
	"os"
	"strings"
	"testing"
)

// TestAccountCreateAndUpdateValidateOrgAndActivationPolicy 验证单账号开通必须校验组织归属和租户激活码策略。
func TestAccountCreateAndUpdateValidateOrgAndActivationPolicy(t *testing.T) {
	raw, err := os.ReadFile("service_account.go")
	if err != nil {
		t.Fatalf("读取账号 service 失败: %v", err)
	}
	source := string(raw)
	createBody := functionBody(t, source, "func (s *Service) CreateAccountByAdmin", "func (s *Service) UpdateAccountByAdmin")
	updateBody := functionBody(t, source, "func (s *Service) UpdateAccountByAdmin", "func (s *Service) ResetAccountPasswordByAdmin")

	if !strings.Contains(createBody, "tx.GetTenantByID") || !strings.Contains(createBody, "EnableActivationCode") {
		t.Fatalf("单账号创建必须读取租户配置并遵守 enable_activation_code")
	}
	if !strings.Contains(createBody, "validateAccountOrgForProfile") {
		t.Fatalf("单账号创建必须按基础身份校验教师院系/学生班级挂靠")
	}
	if !strings.Contains(updateBody, "tx.GetAccount") || !strings.Contains(updateBody, "validateAccountOrgForProfile") {
		t.Fatalf("账号编辑必须读取原账号身份并校验新组织挂靠类型")
	}
}

// TestSensitiveIdentityOperationsWriteAudit 验证权限授予、租户审核和配置变更均写入统一审计。
func TestSensitiveIdentityOperationsWriteAudit(t *testing.T) {
	accountRaw, err := os.ReadFile("service_account.go")
	if err != nil {
		t.Fatalf("读取账号 service 失败: %v", err)
	}
	accountSource := string(accountRaw)
	grantBody := functionBody(t, accountSource, "func (s *Service) GrantSchoolAdmin", "func (s *Service) RevokeSchoolAdmin")
	if !strings.Contains(grantBody, "account.admin.grant") {
		t.Fatalf("授予学校管理员必须写审计")
	}

	platformRaw, err := os.ReadFile("service_platform.go")
	if err != nil {
		t.Fatalf("读取平台 service 失败: %v", err)
	}
	platformSource := string(platformRaw)
	for _, action := range []string{"tenant.application.approve", "tenant.application.reject", "tenant.status.update"} {
		if !strings.Contains(platformSource, action) {
			t.Fatalf("平台敏感操作缺少审计动作: %s", action)
		}
	}

	tenantRaw, err := os.ReadFile("service_tenant.go")
	if err != nil {
		t.Fatalf("读取租户 service 失败: %v", err)
	}
	if !strings.Contains(string(tenantRaw), "tenant.config.update") {
		t.Fatalf("租户配置变更必须写审计")
	}

	authRaw, err := os.ReadFile("service_auth.go")
	if err != nil {
		t.Fatalf("读取认证 service 失败: %v", err)
	}
	authSource := string(authRaw)
	if !strings.Contains(authSource, "auth.login") || !strings.Contains(authSource, "auth.logout") {
		t.Fatalf("登录和登出必须写审计")
	}
}

// TestCASLoginURLUsesLoginEndpoint 验证 CAS 跳转地址显式指向登录端点而不是 CAS 根地址。
func TestCASLoginURLUsesLoginEndpoint(t *testing.T) {
	raw, err := os.ReadFile("service_sso.go")
	if err != nil {
		t.Fatalf("读取 SSO service 失败: %v", err)
	}
	source := string(raw)
	body := functionBody(t, source, "func (s *Service) CASLoginURL", "func (s *Service) CASCallback")
	if !strings.Contains(body, "casLoginURL(") {
		t.Fatalf("CASLoginURL 必须通过统一 helper 构造 /login 跳转地址")
	}
	if !strings.Contains(source, `"/login"`) {
		t.Fatalf("CAS 登录地址必须显式拼接 /login")
	}
}

// TestOrganizationMaintenanceWritesAudit 验证组织架构维护和班级升级属于敏感变更,必须写入统一审计。
func TestOrganizationMaintenanceWritesAudit(t *testing.T) {
	raw, err := os.ReadFile("service_org.go")
	if err != nil {
		t.Fatalf("读取组织 service 失败: %v", err)
	}
	source := string(raw)
	for _, action := range []string{
		"org.department.create",
		"org.department.update",
		"org.department.delete",
		"org.major.create",
		"org.major.update",
		"org.major.delete",
		"org.class.create",
		"org.class.update",
		"org.class.delete",
		"org.class.promote",
	} {
		if !strings.Contains(source, action) {
			t.Fatalf("组织架构敏感操作缺少审计动作: %s", action)
		}
	}
}

// TestOrgImportPreviewValidatesParentExistence 验证组织导入预览阶段就校验上级组织存在,避免提交阶段才吞掉错误。
func TestOrgImportPreviewValidatesParentExistence(t *testing.T) {
	raw, err := os.ReadFile("service_org.go")
	if err != nil {
		t.Fatalf("读取组织 service 失败: %v", err)
	}
	source := string(raw)
	body := functionBody(t, source, "func (s *Service) PreviewOrgImportByAdmin", "func (s *Service) CommitOrgImportByAdmin")
	if !strings.Contains(body, "validateOrgImportParents") {
		t.Fatalf("组织导入预览必须在持久化前校验上级组织存在性")
	}
	if !strings.Contains(source, "tx.DepartmentExists") || !strings.Contains(source, "tx.MajorExists") {
		t.Fatalf("组织导入预览必须分别校验专业所属院系和班级所属专业")
	}
}

// TestOrganizationCRUDValidatesParentTenant 验证专业和班级维护必须显式校验父级属于当前租户。
func TestOrganizationCRUDValidatesParentTenant(t *testing.T) {
	raw, err := os.ReadFile("service_org.go")
	if err != nil {
		t.Fatalf("读取组织 service 失败: %v", err)
	}
	source := string(raw)
	for _, boundary := range []struct {
		start string
		end   string
		want  string
	}{
		{"func (s *Service) CreateMajorByAdmin", "func (s *Service) UpdateMajorByAdmin", "validateMajorParent"},
		{"func (s *Service) UpdateMajorByAdmin", "func (s *Service) DeleteMajorByAdmin", "validateMajorParent"},
		{"func (s *Service) CreateClassByAdmin", "func (s *Service) UpdateClassByAdmin", "validateClassParent"},
		{"func (s *Service) UpdateClassByAdmin", "func (s *Service) DeleteClassByAdmin", "validateClassParent"},
	} {
		body := functionBody(t, source, boundary.start, boundary.end)
		if !strings.Contains(body, boundary.want) {
			t.Fatalf("%s 必须调用 %s 校验父级组织归属", boundary.start, boundary.want)
		}
	}
}

// TestTenantConfigValidatesAuthMode 验证租户配置必须拒绝非法认证模式取值。
func TestTenantConfigValidatesAuthMode(t *testing.T) {
	raw, err := os.ReadFile("service_tenant.go")
	if err != nil {
		t.Fatalf("读取租户 service 失败: %v", err)
	}
	source := string(raw)
	body := functionBody(t, source, "func (s *Service) UpdateTenantConfigByAdmin", "func (s *Service) ListSSOConfigsByAdmin")
	if !strings.Contains(body, "validateTenantConfigRequest") {
		t.Fatalf("租户配置更新必须统一校验 auth_mode 等输入")
	}
	rulesRaw, err := os.ReadFile("rules.go")
	if err != nil {
		t.Fatalf("读取 rules 失败: %v", err)
	}
	if !strings.Contains(string(rulesRaw), "ValidateAuthMode") {
		t.Fatalf("认证模式取值校验应放在 rules.go")
	}
}

// TestPrivateDeployDisablesPlatformRoutes 验证私有化部署不注册平台管理路由,避免仅靠 handler 内部兜底。
func TestPrivateDeployDisablesPlatformRoutes(t *testing.T) {
	raw, err := os.ReadFile("api.go")
	if err != nil {
		t.Fatalf("读取 api.go 失败: %v", err)
	}
	source := string(raw)
	if !strings.Contains(source, "svc.deploy.PlatformEnabled") || !strings.Contains(source, "registerPlatformRoutes") {
		t.Fatalf("RegisterRoutes 必须按部署模式决定是否注册 /platform 路由")
	}
}

// TestPrivateBootstrapServiceExists 验证私有化初始化有 M1 bootstrap 服务入口,不能只保留内部 helper。
func TestPrivateBootstrapServiceExists(t *testing.T) {
	raw, err := os.ReadFile("service_platform.go")
	if err != nil {
		t.Fatalf("读取平台 service 失败: %v", err)
	}
	source := string(raw)
	if !strings.Contains(source, "func (s *Service) BootstrapSchoolTenant") {
		t.Fatalf("identity 必须提供私有化初始化调用的 BootstrapSchoolTenant 服务入口")
	}
	if !strings.Contains(source, "DeployModeSchool") || !strings.Contains(source, "audit.ActorRoleSystem") {
		t.Fatalf("bootstrap 必须创建私有化租户并以系统角色写审计")
	}
}

// TestProtectedRoutesDoNotKeepNilAuthFallback 验证受保护路由不能因 authn 缺失退化为无鉴权注册。
func TestProtectedRoutesDoNotKeepNilAuthFallback(t *testing.T) {
	for _, file := range []string{
		"api_auth.go",
		"api_platform.go",
		"api_tenant.go",
		"api_org.go",
		"api_account.go",
		"api_me.go",
		"api_audit.go",
	} {
		raw, err := os.ReadFile(file)
		if err != nil {
			t.Fatalf("读取 API 文件失败 %s: %v", file, err)
		}
		if strings.Contains(string(raw), "authn != nil") {
			t.Fatalf("%s 不得保留 authn == nil 无鉴权注册分支", file)
		}
	}
}

// TestIdentityReusesPlatformHTTPPageAndPGHelpers 验证 identity 不重复实现平台已有的 HTTP、分页和 pgtype 辅助。
func TestIdentityReusesPlatformHTTPPageAndPGHelpers(t *testing.T) {
	for _, tc := range []struct {
		file string
		bad  []string
	}{
		{file: "api.go", bad: []string{"func bindJSON", "func pathID", "strconv.ParseInt"}},
		{file: "api_account.go", bad: []string{"func queryInt16", "func queryInt32Default", "strconv.ParseInt"}},
		{file: "api_org.go", bad: []string{"func queryInt64", "strconv.ParseInt"}},
		{file: "row_convert.go", bad: []string{"func textValue", "func int8Value", "func int16Value", "func textParam", "func int8Param", "func int2Param", "func timestamptzPtrParam"}},
	} {
		raw, err := os.ReadFile(tc.file)
		if err != nil {
			t.Fatalf("读取生产文件失败 %s: %v", tc.file, err)
		}
		source := string(raw)
		for _, bad := range tc.bad {
			if strings.Contains(source, bad) {
				t.Fatalf("%s 仍重复实现平台已有辅助: %s", tc.file, bad)
			}
		}
	}
}

// TestIdentityReusesPlatformJSONBoundary 验证 identity 生产代码统一复用平台 JSON 边界。
func TestIdentityReusesPlatformJSONBoundary(t *testing.T) {
	files := []string{
		"convert.go",
		"service_import.go",
		"service_org.go",
		"service_sso.go",
		"service_tenant.go",
		"sms_sender.go",
	}
	for _, file := range files {
		raw, err := os.ReadFile(file)
		if err != nil {
			t.Fatalf("读取生产文件失败 %s: %v", file, err)
		}
		source := string(raw)
		for _, bad := range []string{`"encoding/json"`, "json.Marshal", "json.Unmarshal"} {
			if strings.Contains(source, bad) {
				t.Fatalf("%s 不应绕过 platform/jsonx 直接处理 JSON: %s", file, bad)
			}
		}
		if !strings.Contains(source, "jsonx.") {
			t.Fatalf("%s 应显式复用 platform/jsonx 的统一 JSON 语义", file)
		}
	}
}

// TestIdentityServiceDoesNotKeepRuntimeFallbacks 验证 service 不保留测试便利或未装配兜底逻辑。
func TestIdentityServiceDoesNotKeepRuntimeFallbacks(t *testing.T) {
	raw, err := os.ReadFile("service.go")
	if err != nil {
		t.Fatalf("读取 service.go 失败: %v", err)
	}
	source := string(raw)
	for _, bad := range []string{"if s == nil", "time.Now().UTC()"} {
		if strings.Contains(source, bad) {
			t.Fatalf("service.go 不应保留运行期兜底或绕过平台时间工具: %s", bad)
		}
	}
}

// TestComplexProductionFlowsHaveStepComments 验证复杂生产流程具备内部中文步骤注释,避免只写函数头注释。
func TestComplexProductionFlowsHaveStepComments(t *testing.T) {
	cases := []struct {
		file  string
		start string
		end   string
	}{
		{"repo_identity.go", "func (t *txStore) CreateAccount(ctx", "func (t *txStore) GetAccount(ctx"},
		{"repo_identity.go", "func (t *txStore) ListAccounts(ctx", "func (t *txStore) UpdateAccountEditable(ctx"},
		{"repo_identity.go", "func (t *txStore) QueryAuditLogs(ctx", "func (t *txStore) PlatformStats(ctx"},
		{"service_auth.go", "func (s *Service) LoginPlatform(ctx", "func (s *Service) LoginPhone(ctx"},
		{"service_auth.go", "func (s *Service) LoginPhone(ctx", "func (s *Service) LoginNo(ctx"},
		{"service_auth.go", "func (s *Service) LoginSMS(ctx", "func (s *Service) RefreshToken(ctx"},
		{"service_auth.go", "func (s *Service) issueTenantLogin(ctx", "func (s *Service) refreshTenantSession(ctx"},
		{"service_import.go", "func (s *Service) applyAccountImportOpeningRules(ctx", "func accountImportOrgExists(ctx"},
		{"service_me.go", "func (s *Service) ChangeMyPassword(ctx", "func (s *Service) ChangeMyPhone(ctx"},
		{"service_platform.go", "func (s *Service) ApproveApplication(ctx", "func (s *Service) RejectApplication(ctx"},
		{"service_platform.go", "func (s *Service) createBootstrapAdmin(ctx", "func (s *Service) auditPlatformOperation(ctx"},
		{"service_org.go", "func parseOrgImportRecords(records", "func encodeNamedXLSX(sheetName"},
		{"service_sso.go", "func (s *Service) validateLDAPCredentials(ctx", "func (s *Service) decryptLDAPBindPassword(value"},
		{"api_account.go", "func (a accountAPI) importPreview(c", "func (a accountAPI) importTemplate(c"},
	}
	for _, tc := range cases {
		raw, err := os.ReadFile(tc.file)
		if err != nil {
			t.Fatalf("读取生产文件失败 %s: %v", tc.file, err)
		}
		body := functionBody(t, string(raw), tc.start, tc.end)
		if !hasInternalChineseStepComment(body) {
			t.Fatalf("%s 中 %s 缺少内部中文步骤注释", tc.file, tc.start)
		}
	}
}

// functionBody 从源码中截取函数体,用于守护职责和安全关键调用不被删除。
func functionBody(t *testing.T, source, startMarker, endMarker string) string {
	t.Helper()
	start := strings.Index(source, startMarker)
	if start < 0 {
		t.Fatalf("未找到函数: %s", startMarker)
	}
	end := strings.Index(source[start:], endMarker)
	if end < 0 {
		t.Fatalf("未找到函数边界: %s", endMarker)
	}
	return source[start : start+end]
}

// hasInternalChineseStepComment 判断函数体内部是否存在中文步骤注释,排除函数头注释本身。
func hasInternalChineseStepComment(body string) bool {
	lines := strings.Split(body, "\n")
	depth := 0
	for i, line := range lines {
		if strings.Contains(line, "{") {
			depth += strings.Count(line, "{")
		}
		trimmed := strings.TrimSpace(line)
		if i > 0 && depth > 0 && strings.HasPrefix(trimmed, "//") && containsCJK(trimmed) {
			return true
		}
		if strings.Contains(line, "}") {
			depth -= strings.Count(line, "}")
			if depth <= 0 && i > 0 {
				return false
			}
		}
	}
	return false
}

// containsCJK 判断字符串是否包含中文字符,用于注释规范守护测试。
func containsCJK(value string) bool {
	for _, r := range value {
		if r >= '\u4e00' && r <= '\u9fff' {
			return true
		}
	}
	return false
}
