// Package modules 放跨模块架构守护测试,防止通用边界规则在单个模块里回退。
package modules

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestJSONBBoundariesUseModuleErrors 确保 JSONB 业务边界返回模块语义错误码。
func TestJSONBBoundariesUseModuleErrors(t *testing.T) {
	err := filepath.WalkDir(".", func(path string, d os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if d.IsDir() {
			if d.Name() == "sqlcgen" {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		for i, line := range strings.Split(string(data), "\n") {
			if strings.Contains(line, "jsonx.ObjectBytes(") && strings.Contains(line, "apperr.ErrBadRequest") {
				t.Errorf("%s:%d uses apperr.ErrBadRequest at a JSONB business boundary", path, i+1)
			}
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walk modules: %v", err)
	}
}

// TestBusinessConversionsKeepModuleErrorCodes 防止业务类型转换失败退回通用请求错误码。
func TestBusinessConversionsKeepModuleErrorCodes(t *testing.T) {
	err := filepath.WalkDir(".", func(path string, d os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if d.IsDir() {
			if d.Name() == "sqlcgen" {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		for i, line := range strings.Split(string(data), "\n") {
			if strings.Contains(line, "apperr.ErrBadRequest.WithCause(") {
				t.Errorf("%s:%d wraps business conversion with apperr.ErrBadRequest", path, i+1)
			}
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walk modules: %v", err)
	}
}

// TestStructuredColumnConversionsUseJSONX 防止模块转换层自定义 JSONB/结构化列序列化边界。
func TestStructuredColumnConversionsUseJSONX(t *testing.T) {
	for _, path := range []string{
		filepath.Join("contest", "convert.go"),
		filepath.Join("experiment", "convert.go"),
		filepath.Join("content", "rules.go"),
		filepath.Join("identity", "service_contract.go"),
		filepath.Join("identity", "service_import.go"),
		filepath.Join("identity", "service_org.go"),
		filepath.Join("identity", "service_platform.go"),
		filepath.Join("judge", "spec.go"),
		filepath.Join("judge", "service_judgers.go"),
		filepath.Join("sandbox", "service_runtime_admin.go"),
		filepath.Join("sandbox", "spec.go"),
		filepath.Join("sim", "validation.go"),
	} {
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read %s: %v", path, err)
		}
		if strings.Contains(string(data), `"encoding/json"`) {
			t.Errorf("%s imports encoding/json instead of using platform/jsonx", path)
		}
	}
}

// TestAuditWritersDoNotSilentlyBypassMissingWriter 防止高敏感操作在缺少审计 writer 时静默成功。
func TestAuditWritersDoNotSilentlyBypassMissingWriter(t *testing.T) {
	for _, path := range []string{
		filepath.Join("admin", "service.go"),
		filepath.Join("contest", "audit.go"),
		filepath.Join("experiment", "audit.go"),
		filepath.Join("grade", "service.go"),
		filepath.Join("teaching", "audit.go"),
	} {
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read %s: %v", path, err)
		}
		content := strings.ReplaceAll(string(data), "\r\n", "\n")
		if strings.Contains(content, "if s.auditor == nil {\n\t\treturn nil\n\t}") {
			t.Errorf("%s silently bypasses audit when writer is missing", path)
		}
	}
}

// TestAuditRoleResolutionUsesPlatformAudit 防止模块内继续散落重复 actor_role 解析逻辑。
func TestAuditRoleResolutionUsesPlatformAudit(t *testing.T) {
	for _, path := range []string{
		filepath.Join("content", "audit.go"),
		filepath.Join("identity", "audit.go"),
		filepath.Join("identity", "service_account.go"),
		filepath.Join("identity", "service_activation.go"),
		filepath.Join("identity", "service_auth.go"),
		filepath.Join("judge", "audit.go"),
		filepath.Join("sandbox", "audit.go"),
		filepath.Join("sim", "audit.go"),
	} {
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read %s: %v", path, err)
		}
		content := string(data)
		if strings.Contains(content, "func (s *Service) auditRole(") ||
			strings.Contains(content, "func auditRoleFromAccount(") ||
			strings.Contains(content, "func auditRoleFromCodes(") ||
			strings.Contains(content, "auditRoleFromCodes(") {
			t.Errorf("%s keeps module-local audit role mapping instead of platform/audit", path)
		}
	}
}

// TestEventBusDependenciesDoNotSilentlyBypassMissingBus 防止事件发布/订阅链路在缺少总线时静默成功。
func TestEventBusDependenciesDoNotSilentlyBypassMissingBus(t *testing.T) {
	for _, path := range []string{
		filepath.Join("contest", "events.go"),
		filepath.Join("experiment", "events.go"),
		filepath.Join("experiment", "service.go"),
		filepath.Join("grade", "events.go"),
		filepath.Join("notify", "events.go"),
		filepath.Join("teaching", "events.go"),
		filepath.Join("teaching", "service_grade.go"),
	} {
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read %s: %v", path, err)
		}
		content := strings.ReplaceAll(string(data), "\r\n", "\n")
		if strings.Contains(content, "if s.bus == nil {\n\t\treturn nil\n\t}") {
			t.Errorf("%s silently bypasses a missing service bus", path)
		}
		if strings.Contains(content, "if bus == nil {\n\t\treturn nil\n\t}") {
			t.Errorf("%s silently bypasses a missing subscription bus", path)
		}
		if strings.Contains(content, "if s.bus == nil || instance.Score == nil {\n\t\treturn nil\n\t}") {
			t.Errorf("%s silently drops score publication when the event bus is missing", path)
		}
	}
}

// TestCurrentTimeBoundariesUseTimex 防止模块当前时间边界绕过 platform/timex。
func TestCurrentTimeBoundariesUseTimex(t *testing.T) {
	err := filepath.WalkDir(".", func(path string, d os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if d.IsDir() {
			if d.Name() == "sqlcgen" {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		for i, line := range strings.Split(string(data), "\n") {
			if strings.Contains(line, "time.Now()") {
				t.Errorf("%s:%d reads current time directly; use platform/timex.Now at module boundaries", path, i+1)
			}
			if strings.Contains(line, "time.Now().Year()") {
				t.Errorf("%s:%d derives a year from local time; use UTC before building persisted refs", path, i+1)
			}
			if strings.Contains(line, "time.Now().UTC()") {
				t.Errorf("%s:%d reads current UTC time directly; use platform/timex.Now at module boundaries", path, i+1)
			}
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walk modules: %v", err)
	}
}

// TestModulesUsePlatformNoRowsPredicate 防止 repo 直接依赖 pgx.ErrNoRows 判断。
func TestModulesUsePlatformNoRowsPredicate(t *testing.T) {
	err := filepath.WalkDir(".", func(path string, d os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if d.IsDir() {
			if d.Name() == "sqlcgen" {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		for i, line := range strings.Split(string(data), "\n") {
			if strings.Contains(line, "pgx.ErrNoRows") {
				t.Errorf("%s:%d checks pgx.ErrNoRows directly; use platform/db.IsNoRows", path, i+1)
			}
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walk modules: %v", err)
	}
}

// TestPersistedTimeFilesUseTimex 防止持久化过期时间和业务事件时间绕过平台时间入口。
func TestPersistedTimeFilesUseTimex(t *testing.T) {
	for _, path := range []string{
		filepath.Join("identity", "service_activation.go"),
		filepath.Join("identity", "service_auth.go"),
		filepath.Join("identity", "service_import.go"),
		filepath.Join("identity", "service_sms.go"),
		filepath.Join("sandbox", "k8s_orchestrator.go"),
		filepath.Join("sandbox", "service_runtime_admin.go"),
	} {
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read %s: %v", path, err)
		}
		for i, line := range strings.Split(string(data), "\n") {
			if strings.Contains(line, "time.Now()") {
				t.Errorf("%s:%d reads current time directly at a persisted boundary", path, i+1)
			}
		}
	}
}

// TestModulesUsePlatformPagination 防止各模块继续保留本地分页默认值和上限实现。
func TestModulesUsePlatformPagination(t *testing.T) {
	err := filepath.WalkDir(".", func(path string, d os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if d.IsDir() {
			if d.Name() == "sqlcgen" {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		content := string(data)
		if strings.Contains(content, "func normalizePage(") || strings.Contains(content, "func pageParams(") {
			t.Errorf("%s keeps module-local pagination; use platform/pagex.Normalize", path)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walk modules: %v", err)
	}
}

// TestModulesUsePlatformNoRows 防止 repo 重复实现 pgx no rows 判断。
func TestModulesUsePlatformNoRows(t *testing.T) {
	err := filepath.WalkDir(".", func(path string, d os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if d.IsDir() {
			if d.Name() == "sqlcgen" {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		if strings.Contains(string(data), "func isNoRows(") {
			t.Errorf("%s keeps module-local no rows helper; use platform/db.IsNoRows", path)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walk modules: %v", err)
	}
}

// TestModulesUsePlatformIDText 防止模块重复实现雪花 ID 字符串边界。
func TestModulesUsePlatformIDText(t *testing.T) {
	err := filepath.WalkDir(".", func(path string, d os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if d.IsDir() {
			if d.Name() == "sqlcgen" {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		content := string(data)
		for _, sig := range []string{"func fmtID(", "func parseID(", "func parseIDOrZero(", "func judgeParseID(", "func mustOptionalID(", "func mustID("} {
			if strings.Contains(content, sig) {
				t.Errorf("%s keeps module-local ID text helper %s; use platform/ids", path, sig)
			}
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walk modules: %v", err)
	}
}

// TestModulesUseContractRoleHelpers 防止各模块重复维护角色码匹配逻辑。
func TestModulesUseContractRoleHelpers(t *testing.T) {
	err := filepath.WalkDir(".", func(path string, d os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if d.IsDir() {
			if d.Name() == "sqlcgen" {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		if strings.Contains(string(data), "func hasAnyRole(") {
			t.Errorf("%s keeps module-local role matching; use contracts.HasAnyRole", path)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walk modules: %v", err)
	}
}

// TestModulesUsePlatformSecretMap 防止模块重复实现敏感配置加密、脱敏和还原逻辑。
func TestModulesUsePlatformSecretMap(t *testing.T) {
	for _, path := range []string{
		filepath.Join("admin", "config_value.go"),
		filepath.Join("contest", "vuln_config.go"),
		filepath.Join("identity", "service_platform.go"),
	} {
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read %s: %v", path, err)
		}
		content := string(data)
		for _, sig := range []string{"func isSensitiveConfigKey(", "func isVulnSensitiveConfigKey(", "strings.Contains(k, \"password\")"} {
			if strings.Contains(content, sig) {
				t.Errorf("%s keeps module-local sensitive config logic %s; use platform/secretmap", path, sig)
			}
		}
	}
}

// TestModulesUsePlatformStorageRefs 防止模块重复解析 minio://bucket/key 对象引用。
func TestModulesUsePlatformStorageRefs(t *testing.T) {
	for _, path := range []string{
		filepath.Join("judge", "service_worker.go"),
		filepath.Join("sandbox", "service_files.go"),
	} {
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read %s: %v", path, err)
		}
		content := string(data)
		if strings.Contains(content, "func parseJudgeObjectRef(") || strings.Contains(content, "func parseObjectRef(") {
			t.Errorf("%s keeps module-local object ref parser; use platform/storage.ParseObjectRef", path)
		}
	}
}

// TestModulesUsePlatformSourceRefValidation 防止 M2/M3/M4 各自维护 source_ref 格式规则。
func TestModulesUsePlatformSourceRefValidation(t *testing.T) {
	for _, path := range []string{
		filepath.Join("judge", "spec.go"),
		filepath.Join("judge", "service.go"),
		filepath.Join("sandbox", "service.go"),
		filepath.Join("sim", "validation.go"),
		filepath.Join("sim", "service.go"),
	} {
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read %s: %v", path, err)
		}
		content := string(data)
		if strings.Contains(content, "func validateSourceRef(") || strings.Contains(content, "sourceRefRe") {
			t.Errorf("%s keeps module-local source_ref validation; use platform/auth.ValidSourceRef", path)
		}
	}
}

// TestModuleAPIsUseHTTPXBinding 防止 API 层重复实现 JSON 请求体解析和错误响应。
func TestModuleAPIsUseHTTPXBinding(t *testing.T) {
	err := filepath.WalkDir(".", func(path string, d os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if d.IsDir() {
			if d.Name() == "sqlcgen" {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		content := string(data)
		if strings.Contains(content, "ShouldBindJSON(") {
			t.Errorf("%s parses JSON directly; use platform/httpx.BindJSONWithError", path)
		}
		if strings.Contains(content, "httpx.BindJSON(") {
			t.Errorf("%s uses generic request binding; use platform/httpx.BindJSONWithError with the module error code", path)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walk modules: %v", err)
	}
}

// TestModuleBusinessFilesDoNotUseHTTPLayer 防止 service/repo/model/dto/enum 混入 Gin、HTTP 响应或请求绑定职责。
func TestModuleBusinessFilesDoNotUseHTTPLayer(t *testing.T) {
	err := filepath.WalkDir(".", func(path string, d os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if d.IsDir() {
			if d.Name() == "sqlcgen" {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		name := filepath.Base(path)
		if !isBusinessLayerFile(name) {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		content := string(data)
		for _, forbidden := range []string{
			`"github.com/gin-gonic/gin"`,
			"gin.Context",
			"http.ResponseWriter",
			"response.",
			"httpx.Write",
			"httpx.BindJSON",
			"ShouldBindJSON",
		} {
			if strings.Contains(content, forbidden) {
				t.Errorf("%s mixes HTTP layer responsibility via %s", path, forbidden)
			}
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walk modules: %v", err)
	}
}

// TestModuleAPIsDoNotUsePersistenceInfrastructure 防止 API 层越过 service 直接访问数据库、sqlc、对象存储或事件总线。
func TestModuleAPIsDoNotUsePersistenceInfrastructure(t *testing.T) {
	err := filepath.WalkDir(".", func(path string, d os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if d.IsDir() {
			if d.Name() == "sqlcgen" {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		name := filepath.Base(path)
		if !strings.HasPrefix(name, "api") {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		content := string(data)
		for _, forbidden := range []string{
			`"chaimir/internal/platform/db"`,
			`"chaimir/internal/platform/storage"`,
			`"chaimir/internal/platform/eventbus"`,
			"/internal/sqlcgen",
			"sqlcgen.",
			"store.Put",
			"bus.Publish",
		} {
			if strings.Contains(content, forbidden) {
				t.Errorf("%s mixes persistence/infrastructure responsibility via %s", path, forbidden)
			}
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walk modules: %v", err)
	}
}

// TestTeacherAPIRoleGuardsUsePlatformAuth 防止教师侧入口鉴权在多个模块 API 中重复实现。
func TestTeacherAPIRoleGuardsUsePlatformAuth(t *testing.T) {
	for _, path := range []string{
		filepath.Join("content", "api.go"),
		filepath.Join("judge", "api.go"),
		filepath.Join("sim", "api.go"),
		filepath.Join("experiment", "api.go"),
		filepath.Join("contest", "api.go"),
	} {
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read %s: %v", path, err)
		}
		content := string(data)
		if strings.Contains(content, "GetAccount(c.Request.Context(), id.AccountID)") ||
			strings.Contains(content, "HasAnyRole(account.Roles, contracts.RoleTeacher") {
			t.Errorf("%s repeats teacher role lookup; use platform/auth role guard", path)
		}
		if !strings.Contains(content, "auth.RequirePlatformOrAnyRole") &&
			!strings.Contains(content, "auth.AuthorizePlatformOrAnyRole") {
			t.Errorf("%s must delegate teacher API role guards to platform/auth", path)
		}
	}
}

// TestAdminAndPlatformAPIRoleGuardsUsePlatformAuth 防止平台/学校管理员入口鉴权散落在模块 API 中重复实现。
func TestAdminAndPlatformAPIRoleGuardsUsePlatformAuth(t *testing.T) {
	cases := []struct {
		path     string
		required []string
	}{
		{path: filepath.Join("admin", "api.go"), required: []string{"auth.RequireTenantAnyRole"}},
		{path: filepath.Join("grade", "api.go"), required: []string{"auth.RequireTenantAnyRole"}},
		{path: filepath.Join("identity", "api.go"), required: []string{"auth.RequireTenantAnyRole"}},
		{path: filepath.Join("sandbox", "api.go"), required: []string{"auth.RequirePlatformIdentity", "auth.RequirePlatformOrAnyRole"}},
		{path: filepath.Join("judge", "api.go"), required: []string{"auth.RequirePlatformIdentity", "auth.AuthorizePlatformOrAnyRole"}},
		{path: filepath.Join("sim", "api.go"), required: []string{"auth.RequirePlatformIdentity", "auth.RequirePlatformOrAnyRole"}},
		{path: filepath.Join("notify", "api.go"), required: []string{"auth.RequirePlatformOrAnyRole"}},
	}
	for _, tc := range cases {
		data, err := os.ReadFile(tc.path)
		if err != nil {
			t.Fatalf("read %s: %v", tc.path, err)
		}
		content := string(data)
		for _, required := range tc.required {
			if !strings.Contains(content, required) {
				t.Errorf("%s must delegate API role guard to %s", tc.path, required)
			}
		}
		if strings.Contains(content, "HasRole(c.Request.Context(), id.AccountID") {
			t.Errorf("%s repeats API role lookup; use platform/auth role guards", tc.path)
		}
		if strings.Contains(content, "func (a *API) requireRole") {
			t.Errorf("%s repeats tenant role middleware; use platform/auth role guards", tc.path)
		}
	}
}

// TestPlatformAuthDoesNotDependOnContracts 防止基础鉴权层反向依赖模块间契约 DTO。
func TestPlatformAuthDoesNotDependOnContracts(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("..", "platform", "auth", "middleware.go"))
	if err != nil {
		t.Fatalf("read platform auth middleware: %v", err)
	}
	if strings.Contains(string(data), `"chaimir/internal/contracts"`) {
		t.Fatalf("platform/auth must depend only on a minimal local role-checking interface, not internal/contracts")
	}
}

// TestModuleEventsUseEventbusDecode 防止事件订阅回调重复实现 JSON 解码和错误码包装。
func TestModuleEventsUseEventbusDecode(t *testing.T) {
	err := filepath.WalkDir(".", func(path string, d os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if d.IsDir() {
			if d.Name() == "sqlcgen" {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, "events.go") {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		content := string(data)
		if strings.Contains(content, "json.Unmarshal(data") {
			t.Errorf("%s decodes event payloads directly; use platform/eventbus.Decode", path)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walk module events: %v", err)
	}
}

// TestModuleEventsDoNotUsePersistenceInfrastructure 防止事件文件越过 service/repo 直接执行持久化写入。
func TestModuleEventsDoNotUsePersistenceInfrastructure(t *testing.T) {
	err := filepath.WalkDir(".", func(path string, d os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if d.IsDir() {
			if d.Name() == "sqlcgen" {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, "events.go") {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		content := string(data)
		for _, forbidden := range []string{
			`"chaimir/internal/platform/db"`,
			"/internal/sqlcgen",
			"sqlcgen.",
			".repo.in",
			".store.in",
			"s.store.",
			"q.Create",
			"q.Update",
			"q.Delete",
		} {
			if strings.Contains(content, forbidden) {
				t.Errorf("%s mixes event handling and persistence responsibility via %s", path, forbidden)
			}
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walk module events: %v", err)
	}
}

// TestModuleServicesDoNotUseSQLCGenDirectly 防止 service/worker/helper 绕过 repo 承担数据访问或事务职责。
func TestModuleServicesDoNotUseSQLCGenDirectly(t *testing.T) {
	err := filepath.WalkDir(".", func(path string, d os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if d.IsDir() {
			if d.Name() == "sqlcgen" {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		name := filepath.Base(path)
		if isDataAccessBoundaryFile(name) {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		content := string(data)
		for _, forbidden := range []string{
			"/internal/sqlcgen",
			"sqlcgen.Queries",
			"sqlcgen.New(",
		} {
			if strings.Contains(content, forbidden) {
				t.Errorf("%s mixes service/business responsibility with sqlc data access via %s", path, forbidden)
			}
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walk modules: %v", err)
	}
}

// TestReposDoNotExposeSQLCRows 防止 repo 对 service 暴露 sqlc 生成类型。
func TestReposDoNotExposeSQLCRows(t *testing.T) {
	err := filepath.WalkDir(".", func(path string, d os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if d.IsDir() {
			if d.Name() == "sqlcgen" {
				return filepath.SkipDir
			}
			return nil
		}
		name := filepath.Base(path)
		if !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") || !isDataAccessBoundaryFile(name) {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		for i, line := range strings.Split(string(data), "\n") {
			trimmed := strings.TrimSpace(line)
			if strings.HasPrefix(trimmed, "func (r *repo)") &&
				(strings.Contains(trimmed, ") sqlcgen.") ||
					strings.Contains(trimmed, ") []sqlcgen.") ||
					strings.Contains(trimmed, ") (sqlcgen.") ||
					strings.Contains(trimmed, ") ([]sqlcgen.") ||
					strings.Contains(trimmed, ", sqlcgen.") ||
					strings.Contains(trimmed, ", []sqlcgen.")) {
				t.Errorf("%s:%d exposes sqlcgen type in repo method signature; map it before returning to service", path, i+1)
			}
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walk modules: %v", err)
	}
}

// TestModelFilesDoNotAliasDTOOrContractTypes 防止 model 用类型别名制造同字段伪领域模型。
func TestModelFilesDoNotAliasDTOOrContractTypes(t *testing.T) {
	err := filepath.WalkDir(".", func(path string, d os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if d.IsDir() {
			if d.Name() == "sqlcgen" {
				return filepath.SkipDir
			}
			return nil
		}
		if filepath.Base(path) != "model.go" {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		for i, line := range strings.Split(string(data), "\n") {
			trimmed := strings.TrimSpace(line)
			if isModelAliasToBoundaryType(trimmed) {
				t.Errorf("%s:%d aliases DTO/contracts in model; use the existing unified type or define a real internal snapshot", path, i+1)
			}
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walk module models: %v", err)
	}
}

// isModelAliasToBoundaryType 判断 model 类型声明是否只是边界类型的别名或同名重定义。
func isModelAliasToBoundaryType(line string) bool {
	if !strings.HasPrefix(line, "type ") {
		return false
	}
	fields := strings.Fields(line)
	if len(fields) < 3 {
		return false
	}
	target := fields[2]
	if target == "=" && len(fields) >= 4 {
		target = fields[3]
	}
	return strings.HasSuffix(target, "DTO") || strings.HasPrefix(target, "contracts.")
}

// TestServiceLayerFileNamesUsePrefix 防止服务层拆分文件使用后缀命名导致职责扫描漏检。
func TestServiceLayerFileNamesUsePrefix(t *testing.T) {
	err := filepath.WalkDir(".", func(path string, d os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if d.IsDir() {
			if d.Name() == "sqlcgen" {
				return filepath.SkipDir
			}
			return nil
		}
		name := filepath.Base(path)
		if !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			return nil
		}
		if strings.HasSuffix(name, "_service.go") {
			t.Errorf("%s uses service suffix naming; use service_<domain>.go so service scans include it", path)
		}
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read %s: %v", path, err)
		}
		if strings.Contains(string(data), "func (s *Service)") && !isServiceLayerFileName(name) {
			t.Errorf("%s contains Service methods but is not named service_<domain>.go", path)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walk modules: %v", err)
	}
}

// isServiceLayerFileName 判断文件名是否属于服务层统一命名或明确边界例外。
func isServiceLayerFileName(name string) bool {
	if name == "service.go" || strings.HasPrefix(name, "service_") {
		return true
	}
	return name == "audit.go" || name == "events.go"
}

// TestProductionCodeHasNoTodoOrPlaceholderMarkers 防止生产代码留下 TODO、占位或测试替身语义。
func TestProductionCodeHasNoTodoOrPlaceholderMarkers(t *testing.T) {
	forbidden := []string{"TODO", "FIXME", "placeholder", "stub", "mock", "fake", "简化", "占位"}
	err := filepath.WalkDir(".", func(path string, d os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if d.IsDir() {
			if d.Name() == "sqlcgen" {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		for i, line := range strings.Split(string(data), "\n") {
			for _, marker := range forbidden {
				if strings.Contains(line, marker) {
					t.Errorf("%s:%d contains production marker %q", path, i+1, marker)
				}
			}
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walk modules: %v", err)
	}
}

// TestModulesDoNotImportOtherModulePackages 防止模块绕过 internal/contracts 直接依赖其他模块。
func TestModulesDoNotImportOtherModulePackages(t *testing.T) {
	moduleNames := map[string]bool{
		"identity": true, "sandbox": true, "judge": true, "sim": true, "content": true, "teaching": true,
		"experiment": true, "contest": true, "admin": true, "notify": true, "grade": true,
	}
	err := filepath.WalkDir(".", func(path string, d os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if d.IsDir() {
			if d.Name() == "sqlcgen" {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		parts := strings.Split(filepath.ToSlash(path), "/")
		if len(parts) < 2 || !moduleNames[parts[0]] {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		for _, line := range strings.Split(string(data), "\n") {
			trimmed := strings.TrimSpace(line)
			if !strings.HasPrefix(trimmed, `"chaimir/internal/modules/`) {
				continue
			}
			importPath := strings.Trim(trimmed, `"`)
			rest := strings.TrimPrefix(importPath, "chaimir/internal/modules/")
			targetModule := strings.Split(rest, "/")[0]
			if targetModule != parts[0] {
				t.Errorf("%s imports module %s directly; use internal/contracts", path, targetModule)
			}
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walk modules: %v", err)
	}
}

// TestBackendProductionFilesHaveHeaderComments 防止生产文件缺少开头职责说明。
func TestBackendProductionFilesHaveHeaderComments(t *testing.T) {
	root := filepath.Join("..", "..", "..")
	err := filepath.WalkDir(root, func(path string, d os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if d.IsDir() {
			switch d.Name() {
			case ".gocache", "sqlcgen":
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		for i, line := range strings.Split(string(data), "\n") {
			trimmed := strings.TrimSpace(line)
			if trimmed == "" {
				continue
			}
			if !isCommentLine(trimmed) {
				t.Errorf("%s:%d production file lacks a header responsibility comment", path, i+1)
			}
			if !hasChinese(trimmed) {
				t.Errorf("%s:%d production file header comment must be written in Chinese", path, i+1)
			}
			return nil
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walk backend: %v", err)
	}
}

// TestBackendProductionFunctionsHaveComments 防止生产函数缺少职责说明。
func TestBackendProductionFunctionsHaveComments(t *testing.T) {
	root := filepath.Join("..", "..", "..")
	err := filepath.WalkDir(root, func(path string, d os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if d.IsDir() {
			switch d.Name() {
			case ".gocache", "sqlcgen":
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		lines := strings.Split(string(data), "\n")
		for i, line := range lines {
			if !isFunctionLine(line) {
				continue
			}
			j := i - 1
			for j >= 0 && strings.TrimSpace(lines[j]) == "" {
				j--
			}
			if j < 0 || !isCommentLine(lines[j]) {
				t.Errorf("%s:%d production function lacks a responsibility comment", path, i+1)
				continue
			}
			if !hasChinese(lines[j]) {
				t.Errorf("%s:%d production function comment must be written in Chinese", path, i+1)
			}
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walk backend: %v", err)
	}
}

// isFunctionLine 识别顶层函数或方法声明,用于注释守护测试。
func isFunctionLine(line string) bool {
	trimmed := strings.TrimSpace(line)
	return strings.HasPrefix(trimmed, "func ") && strings.Contains(trimmed, "(")
}

// isCommentLine 判断上一条非空行是否为 Go 注释。
func isCommentLine(line string) bool {
	trimmed := strings.TrimSpace(line)
	return strings.HasPrefix(trimmed, "//") || strings.HasPrefix(trimmed, "/*")
}

// hasChinese 判断注释中是否包含中文字符。
func hasChinese(line string) bool {
	for _, r := range line {
		if r >= '\u4e00' && r <= '\u9fff' {
			return true
		}
	}
	return false
}

// isBusinessLayerFile 判断模块内文件是否属于非 HTTP 边界层。
func isBusinessLayerFile(name string) bool {
	return name == "repo.go" ||
		name == "model.go" ||
		name == "dto.go" ||
		name == "enum.go" ||
		name == "convert.go" ||
		name == "rules.go" ||
		strings.Contains(name, "service")
}

// isDataAccessBoundaryFile 判断允许触碰 sqlcgen 的模块数据边界文件。
func isDataAccessBoundaryFile(name string) bool {
	return name == "repo.go" ||
		name == "row_convert.go" ||
		name == "audit.go" ||
		strings.HasPrefix(name, "repo_") ||
		strings.HasSuffix(name, "_repo.go")
}

// TestContentUsesContractRoleConstants 防止内容模块把角色码硬编码在权限判断中。
func TestContentUsesContractRoleConstants(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("content", "rules.go"))
	if err != nil {
		t.Fatalf("read content rules: %v", err)
	}
	if strings.Contains(string(data), `"school_admin"`) {
		t.Fatalf("content rules must use contracts role constants instead of hard-coded role strings")
	}
}

// TestDTOAndEnumFilesDoNotContainFunctions 防止 DTO/枚举文件混入转换、校验或业务逻辑职责。
func TestDTOAndEnumFilesDoNotContainFunctions(t *testing.T) {
	err := filepath.WalkDir(".", func(path string, d os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if d.IsDir() {
			if d.Name() == "sqlcgen" {
				return filepath.SkipDir
			}
			return nil
		}
		name := filepath.Base(path)
		if name != "dto.go" && name != "enum.go" {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		for i, line := range strings.Split(string(data), "\n") {
			if isFunctionLine(line) {
				t.Errorf("%s:%d keeps function logic in %s; move it to a responsibility-specific file", path, i+1, name)
			}
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walk modules: %v", err)
	}
}
