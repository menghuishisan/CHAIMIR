// M2 沙箱服务纯规则测试:覆盖 namespace、source_ref、工具生态适配等不依赖外部系统的边界。
package sandbox

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"os"
	"strings"
	"testing"

	"chaimir/internal/contracts"
	"chaimir/internal/platform/config"
)

// TestSandboxNamespaceUsesConfiguredPrefix 确认 Namespace 命名只由配置前缀与沙箱 ID 组成。
func TestSandboxNamespaceUsesConfiguredPrefix(t *testing.T) {
	got := sandboxNamespace("sbx", 9001)
	if got != "sbx-9001" {
		t.Fatalf("unexpected namespace: %s", got)
	}
}

// TestGetSandboxUsesOwnerCheckedLoader 确认沙箱摘要查询复用 owner 校验,避免同租户横向查看。
func TestGetSandboxUsesOwnerCheckedLoader(t *testing.T) {
	data, err := os.ReadFile("service.go")
	if err != nil {
		t.Fatalf("read service.go: %v", err)
	}
	body := string(data)
	start := strings.Index(body, "func (s *Service) GetSandbox(")
	end := strings.Index(body, "// RecycleBySourceRef")
	if start < 0 || end < start {
		t.Fatalf("GetSandbox function block not found")
	}
	if !strings.Contains(body[start:end], "s.loadSandboxRow(ctx, sandboxID)") {
		t.Fatalf("GetSandbox must use loadSandboxRow so owner_account_id is checked")
	}
}

// TestValidateSourceRefRequiresDocumentedShape 确认来源标识符合 <来源>:<年份>:<资源类型>:<id>。
func TestValidateSourceRefRequiresDocumentedShape(t *testing.T) {
	if err := validateSourceRef("exp:2026:instance:55"); err != nil {
		t.Fatalf("valid source_ref rejected: %v", err)
	}
	if err := validateSourceRef("exp:55"); err == nil {
		t.Fatalf("invalid source_ref should be rejected")
	}
}

// TestToolFitsRuntimeEco 确认工具按生态标签匹配运行时。
func TestToolFitsRuntimeEco(t *testing.T) {
	if !toolFitsRuntimeEco("evm,fabric", "evm") {
		t.Fatalf("evm tool should match evm runtime")
	}
	if toolFitsRuntimeEco("fabric", "evm") {
		t.Fatalf("fabric-only tool must not match evm runtime")
	}
}

// TestSandboxWorkspacePathRejectsTraversal 确认文件接口不会跳出运行时工作目录。
func TestSandboxWorkspacePathRejectsTraversal(t *testing.T) {
	target, err := sandboxWorkspacePath("/workspace", "contracts/Counter.sol")
	if err != nil {
		t.Fatalf("expected regular path to be accepted: %v", err)
	}
	if target != "/workspace/contracts/Counter.sol" {
		t.Fatalf("unexpected workspace path: %s", target)
	}

	if _, err := sandboxWorkspacePath("/workspace", "../../../etc/passwd"); err == nil {
		t.Fatalf("expected traversal path to be rejected")
	}
	if _, err := sandboxWorkspacePath("/workspace", "/etc/passwd"); err == nil {
		t.Fatalf("expected absolute path outside workspace to be rejected")
	}
}

// TestSafeSandboxInitArchiveRejectsTraversal 确认初始代码归档在进入容器前完成后端安全重打包。
func TestSafeSandboxInitArchiveRejectsTraversal(t *testing.T) {
	cfg := config.SandboxConfig{InitArchiveMaxFiles: 10, InitArchiveMaxUnpackedBytes: 1024}
	if _, err := safeSandboxInitArchive(testSandboxArchive(t, map[string]string{
		"src/main.sol": "contract C {}",
	}), cfg); err != nil {
		t.Fatalf("safe init archive rejected: %v", err)
	}
	if _, err := safeSandboxInitArchive(testSandboxArchive(t, map[string]string{
		"../escape": "pwn",
	}), cfg); err == nil {
		t.Fatalf("archive traversal entry must be rejected before container tar extraction")
	}
}

// TestSafeSandboxInitArchiveAppliesConfiguredLimits 确认归档文件数和展开大小来自 SandboxConfig。
func TestSafeSandboxInitArchiveAppliesConfiguredLimits(t *testing.T) {
	if _, err := safeSandboxInitArchive(testSandboxArchive(t, map[string]string{
		"a.txt": "a",
		"b.txt": "b",
	}), config.SandboxConfig{InitArchiveMaxFiles: 1, InitArchiveMaxUnpackedBytes: 1024}); err == nil {
		t.Fatalf("archive with too many files must be rejected")
	}
	if _, err := safeSandboxInitArchive(testSandboxArchive(t, map[string]string{
		"a.txt": "abcd",
	}), config.SandboxConfig{InitArchiveMaxFiles: 10, InitArchiveMaxUnpackedBytes: 3}); err == nil {
		t.Fatalf("archive exceeding unpacked byte limit must be rejected")
	}
}

// TestToolProxyTargetRequiresSandboxWebTool 防止用户通过工具代理访问 runtime 暴露的非工具端口。
func TestToolProxyTargetRequiresSandboxWebTool(t *testing.T) {
	data, err := os.ReadFile("interaction.go")
	if err != nil {
		t.Fatalf("read interaction.go: %v", err)
	}
	body := string(data)
	start := strings.Index(body, "func (s *Service) ToolProxyTarget(")
	end := strings.Index(body, "// loadSandboxRow")
	if start < 0 || end < start {
		t.Fatalf("ToolProxyTarget function block not found")
	}
	block := body[start:end]
	if !strings.Contains(block, "GetSandboxToolForProxy") || !strings.Contains(block, "ToolKindWebEmbed") {
		t.Fatalf("ToolProxyTarget must verify the requested code is a mounted web tool before proxying")
	}
}

// TestWorkspaceReadCommandUsesRealpathGuard 确认容器内文件访问会拒绝 symlink 逃逸。
func TestWorkspaceReadCommandUsesRealpathGuard(t *testing.T) {
	command := workspaceReadCommand("/workspace", "/workspace/contracts/Counter.sol")
	if len(command) != 3 || !strings.Contains(command[2], "realpath -m") || !strings.Contains(command[2], "case \"$resolved\" in") {
		t.Fatalf("read command must guard resolved path, got %#v", command)
	}
}

func testSandboxArchive(t *testing.T, files map[string]string) []byte {
	t.Helper()
	var raw bytes.Buffer
	gz := gzip.NewWriter(&raw)
	tw := tar.NewWriter(gz)
	for name, body := range files {
		data := []byte(body)
		if err := tw.WriteHeader(&tar.Header{Name: name, Mode: 0o644, Size: int64(len(data))}); err != nil {
			t.Fatalf("write tar header: %v", err)
		}
		if _, err := tw.Write(data); err != nil {
			t.Fatalf("write tar body: %v", err)
		}
	}
	if err := tw.Close(); err != nil {
		t.Fatalf("close tar: %v", err)
	}
	if err := gz.Close(); err != nil {
		t.Fatalf("close gzip: %v", err)
	}
	return raw.Bytes()
}

// TestValidateQuotaRequestRequiresKeepaliveAndSnapshotLimits 确认配额包含保活和快照上限,避免教师开启后无边界占用资源。
func TestValidateQuotaRequestRequiresKeepaliveAndSnapshotLimits(t *testing.T) {
	req := QuotaRequest{
		MaxConcurrentSandbox:    10,
		MaxCPU:                  100,
		MaxMemoryMB:             102400,
		IdleTimeoutMin:          30,
		MaxLifetimeMin:          240,
		MaxKeepaliveMin:         120,
		MaxSnapshotRetentionMin: 1440,
	}
	if err := validateQuotaRequest(req); err != nil {
		t.Fatalf("valid quota rejected: %v", err)
	}

	req.MaxSnapshotRetentionMin = 0
	if err := validateQuotaRequest(req); err == nil {
		t.Fatalf("quota without snapshot retention limit must be rejected")
	}
}

// TestValidateCreateSandboxRequestChecksKeepaliveAndSnapshotDurations 确认创建请求中的保活/快照时长不能绕过配额上限语义。
func TestValidateCreateSandboxRequestChecksKeepaliveAndSnapshotDurations(t *testing.T) {
	req := validSandboxCreateRequest()
	req.KeepAlive = true
	req.KeepAliveMinutes = 30
	req.SnapshotEnabled = true
	req.SnapshotRetentionMinutes = 60
	if err := validateCreateSandboxRequest(req); err != nil {
		t.Fatalf("valid create request rejected: %v", err)
	}

	req.KeepAliveMinutes = 0
	if err := validateCreateSandboxRequest(req); err == nil {
		t.Fatalf("keep_alive=true without keep_alive_minutes must be rejected")
	}
}

// TestCreateSandboxChecksKeepaliveAgainstMaxLifetime 确认保活时长同时受最长生命周期约束,不能只检查 max_keepalive_min。
func TestCreateSandboxChecksKeepaliveAgainstMaxLifetime(t *testing.T) {
	data, err := os.ReadFile("service.go")
	if err != nil {
		t.Fatalf("read service.go: %v", err)
	}
	body := string(data)
	start := strings.Index(body, "func (s *Service) CreateSandbox(")
	end := strings.Index(body, "// GetSandbox")
	if start < 0 || end < start {
		t.Fatalf("CreateSandbox function block not found")
	}
	block := body[start:end]
	if !strings.Contains(block, "req.KeepAliveMinutes > quota.MaxLifetimeMin") {
		t.Fatalf("keep_alive must be capped by tenant_quota.max_lifetime_min")
	}
}

// TestCreateSandboxRequiresPreparedDefaultImage 确认创建沙箱前拒绝未预拉取或未固化创世态的默认镜像。
func TestCreateSandboxRequiresPreparedDefaultImage(t *testing.T) {
	data, err := os.ReadFile("service.go")
	if err != nil {
		t.Fatalf("read service.go: %v", err)
	}
	body := string(data)
	start := strings.Index(body, "func (s *Service) CreateSandbox(")
	end := strings.Index(body, "// GetSandbox")
	if start < 0 || end < start {
		t.Fatalf("CreateSandbox function block not found")
	}
	block := body[start:end]
	if !strings.Contains(block, "!image.Prepulled") || !strings.Contains(block, "image.PrepullStatus != RuntimeImagePrepullDone") {
		t.Fatalf("CreateSandbox must reject default runtime images before DaemonSet prepull is complete")
	}
	if !strings.Contains(block, "!image.GenesisBaked") {
		t.Fatalf("CreateSandbox must reject default runtime images without baked genesis state")
	}
}

// TestCreateSandboxSupportsPinnedRuntimeImageVersion 确认内部调用方可按固定运行时镜像版本创建可复现沙箱。
func TestCreateSandboxSupportsPinnedRuntimeImageVersion(t *testing.T) {
	data, err := os.ReadFile("service.go")
	if err != nil {
		t.Fatalf("read service.go: %v", err)
	}
	body := string(data)
	start := strings.Index(body, "func (s *Service) CreateSandbox(")
	end := strings.Index(body, "// GetSandbox")
	if start < 0 || end < start {
		t.Fatalf("CreateSandbox function block not found")
	}
	block := body[start:end]
	if !strings.Contains(block, "GetRuntimeImageByVersion") {
		t.Fatalf("CreateSandbox must resolve req.RuntimeImageVersion instead of always using current default image")
	}

	contractSrc, err := os.ReadFile("../../contracts/engine.go")
	if err != nil {
		t.Fatalf("read contracts engine: %v", err)
	}
	if !strings.Contains(string(contractSrc), "RuntimeImageVersion") {
		t.Fatalf("SandboxCreateRequest/SandboxInfo must expose runtime image version for deterministic engines")
	}
}

// TestCreateSandboxChecksSnapshotCapability 确认 snapshot_enabled 创建时先检查 CSI 快照能力。
func TestCreateSandboxChecksSnapshotCapability(t *testing.T) {
	data, err := os.ReadFile("service.go")
	if err != nil {
		t.Fatalf("read service.go: %v", err)
	}
	body := string(data)
	start := strings.Index(body, "func (s *Service) CreateSandbox(")
	end := strings.Index(body, "// GetSandbox")
	if start < 0 || end < start {
		t.Fatalf("CreateSandbox function block not found")
	}
	block := body[start:end]
	if !strings.Contains(block, "SnapshotAvailable(ctx)") {
		t.Fatalf("snapshot_enabled creation must check orchestrator snapshot capability before persisting sandbox")
	}
}

// TestCreateSandboxChecksTenantCPUAndMemoryQuota 确认创建沙箱同时检查租户 CPU/内存总量配额。
func TestCreateSandboxChecksTenantCPUAndMemoryQuota(t *testing.T) {
	serviceSrc, err := os.ReadFile("service.go")
	if err != nil {
		t.Fatalf("read service.go: %v", err)
	}
	serviceBody := string(serviceSrc)
	start := strings.Index(serviceBody, "func (s *Service) CreateSandbox(")
	end := strings.Index(serviceBody, "// GetSandbox")
	if start < 0 || end < start {
		t.Fatalf("CreateSandbox function block not found")
	}
	if !strings.Contains(serviceBody[start:end], "checkTenantResourceQuota") {
		t.Fatalf("CreateSandbox must check CPU/memory quota before persisting sandbox")
	}

	querySrc, err := os.ReadFile("../../../db/queries/sandbox.sql")
	if err != nil {
		t.Fatalf("read sandbox.sql: %v", err)
	}
	if !strings.Contains(string(querySrc), "ListActiveSandboxResourceSpecs") {
		t.Fatalf("sandbox queries must expose active sandbox resource specs for tenant CPU/memory quota")
	}
}

// TestStageOneFailureTriggersK8sRecycle 确认阶段一失败进入 error 后会清理已创建的 K8s 资源。
func TestStageOneFailureTriggersK8sRecycle(t *testing.T) {
	data, err := os.ReadFile("service.go")
	if err != nil {
		t.Fatalf("read service.go: %v", err)
	}
	body := string(data)
	start := strings.Index(body, "func (s *Service) recordAsyncSandboxError(")
	end := strings.Index(body, "// recordAsyncSandboxInitializationError")
	if start < 0 || end < start {
		t.Fatalf("recordAsyncSandboxError function block not found")
	}
	block := body[start:end]
	if !strings.Contains(block, "s.orchestrator.Recycle(ctx, spec.Namespace)") {
		t.Fatalf("stage-one startup failure must recycle K8s resources after marking sandbox error")
	}
}

// TestResumeUsesSandboxRecordedImage 确认恢复沙箱使用实例记录的 image_id,而不是运行时当前默认镜像。
func TestResumeUsesSandboxRecordedImage(t *testing.T) {
	data, err := os.ReadFile("interaction.go")
	if err != nil {
		t.Fatalf("read interaction.go: %v", err)
	}
	body := string(data)
	start := strings.Index(body, "func (s *Service) loadSandboxDependencies(")
	if start < 0 {
		t.Fatalf("loadSandboxDependencies function block not found")
	}
	block := body[start:]
	if strings.Contains(block, "GetDefaultRuntimeImage(ctx, row.RuntimeID)") {
		t.Fatalf("resume/dependency loading must not switch to the current default image")
	}
	if !strings.Contains(block, "GetRuntimeImage(ctx, sqlcgen.GetRuntimeImageParams{ID: row.ImageID, RuntimeID: row.RuntimeID})") {
		t.Fatalf("resume/dependency loading must use sandbox.image_id")
	}
}

// TestPauseAndResumeWriteAudit 确认暂停/恢复关键操作写入 M1 audit_log,不能只写 sandbox_event 或进度状态。
func TestPauseAndResumeWriteAudit(t *testing.T) {
	interactionSrc, err := os.ReadFile("interaction.go")
	if err != nil {
		t.Fatalf("read interaction.go: %v", err)
	}
	interactionBody := string(interactionSrc)
	for _, tc := range []struct {
		name   string
		start  string
		end    string
		action string
	}{
		{
			name:   "pause",
			start:  "func (s *Service) PauseSandbox(",
			end:    "// ResumeSandbox",
			action: "auditActionSandboxPause",
		},
		{
			name:   "resume",
			start:  "func (s *Service) ResumeSandbox(",
			end:    "// ToolProxyTarget",
			action: "auditActionSandboxResume",
		},
	} {
		start := strings.Index(interactionBody, tc.start)
		end := strings.Index(interactionBody, tc.end)
		if start < 0 || end < start {
			t.Fatalf("%s function block not found", tc.name)
		}
		block := interactionBody[start:end]
		if !strings.Contains(block, "s.writeAudit(") || !strings.Contains(block, tc.action) {
			t.Fatalf("%s must write audit_log with %s after successful state change", tc.name, tc.action)
		}
	}

	auditSrc, err := os.ReadFile("audit.go")
	if err != nil {
		t.Fatalf("read audit.go: %v", err)
	}
	auditBody := string(auditSrc)
	if !strings.Contains(auditBody, `auditActionSandboxPause`) ||
		!strings.Contains(auditBody, `auditActionSandboxResume`) {
		t.Fatalf("audit.go must centrally define sandbox pause/resume action codes")
	}
}

// TestInteractivePathsCheckSandboxLifecycleState 确认终端、文件、工具和链能力在绑定 K8s 前校验生命周期状态。
func TestInteractivePathsCheckSandboxLifecycleState(t *testing.T) {
	filesSrc, err := os.ReadFile("files.go")
	if err != nil {
		t.Fatalf("read files.go: %v", err)
	}
	filesBody := string(filesSrc)
	start := strings.Index(filesBody, "func (s *Service) runtimeBindingForSandbox(")
	end := strings.Index(filesBody, "// runtimeBindingForSandboxRow")
	if start < 0 || end < start {
		t.Fatalf("runtimeBindingForSandbox function block not found")
	}
	if !strings.Contains(filesBody[start:end], "ensureSandboxInteractive(current.Status)") {
		t.Fatalf("runtimeBindingForSandbox must validate sandbox lifecycle state before K8s binding")
	}

	serviceSrc, err := os.ReadFile("service.go")
	if err != nil {
		t.Fatalf("read service.go: %v", err)
	}
	serviceBody := string(serviceSrc)
	start = strings.Index(serviceBody, "func (s *Service) capabilityForSandbox(")
	end = strings.Index(serviceBody, "// markSandboxError")
	if start < 0 || end < start {
		t.Fatalf("capabilityForSandbox function block not found")
	}
	if !strings.Contains(serviceBody[start:end], "ensureSandboxInteractive(row.Status)") {
		t.Fatalf("capabilityForSandbox must validate sandbox lifecycle state before chain capability")
	}
}

// TestParseRuntimeAdapterSpecRejectsInvalidResourceQuantity 确认坏资源声明在边界处失败,不会进入 K8s MustParse 路径。
func TestParseRuntimeAdapterSpecRejectsInvalidResourceQuantity(t *testing.T) {
	raw := []byte(`{
		"workspace_dir": "/workspace",
		"runtime_container": {
			"name": "runtime",
			"command": ["anvil"],
			"resources": {
				"requests": { "cpu": "bad-cpu", "memory": "256Mi" },
				"limits": { "cpu": "1", "memory": "1Gi" }
			}
		}
	}`)

	if _, err := parseRuntimeAdapterSpec(raw); err == nil {
		t.Fatalf("invalid resource quantity must be rejected")
	}
}

// TestParseToolResourceSpecUsesToolErrorCode 确认工具声明错误不会复用运行时错误码。
func TestParseToolResourceSpecUsesToolErrorCode(t *testing.T) {
	tool := ToolDefinition{Kind: ToolKindWebEmbed, Port: 13337}
	_, err := parseToolResourceSpec(tool, []byte(`{"command":["code-server"],"resources":{"requests":{"cpu":"bad-cpu"}}}`))
	if err == nil {
		t.Fatalf("invalid tool resource spec must fail")
	}
	if !strings.Contains(err.Error(), "23004") {
		t.Fatalf("expected ErrToolCreateInvalid, got %v", err)
	}
}

// TestCreateToolInvalidRequestUsesDedicatedCode 确认工具注册参数错误返回 23004,不是工具不存在。
func TestCreateToolInvalidRequestUsesDedicatedCode(t *testing.T) {
	svc := &Service{}
	_, err := svc.CreateTool(nil, CreateToolRequest{Name: "bad", Kind: ToolKindWebEmbed, EcoTags: "evm"})
	if err == nil {
		t.Fatalf("invalid tool request must fail")
	}
	if err.Error() == "" || !strings.Contains(err.Error(), "23004") {
		t.Fatalf("expected ErrToolCreateInvalid, got %v", err)
	}
}

// TestRuntimeAndToolAdminValidateSpecsBeforePersistence 确认平台管理入口在写库前校验声明式清单与工具资源。
func TestRuntimeAndToolAdminValidateSpecsBeforePersistence(t *testing.T) {
	serviceSrc, err := os.ReadFile("service.go")
	if err != nil {
		t.Fatalf("read service.go: %v", err)
	}
	serviceBody := string(serviceSrc)
	createRuntimeStart := strings.Index(serviceBody, "func (s *Service) CreateRuntime(")
	createRuntimeEnd := strings.Index(serviceBody, "// CreateRuntimeImage")
	if createRuntimeStart < 0 || createRuntimeEnd < createRuntimeStart {
		t.Fatalf("CreateRuntime function block not found")
	}
	if !strings.Contains(serviceBody[createRuntimeStart:createRuntimeEnd], "parseRuntimeAdapterSpec(spec)") {
		t.Fatalf("CreateRuntime must validate adapter_spec before persistence")
	}
	createToolStart := strings.Index(serviceBody, "func (s *Service) CreateTool(")
	createToolEnd := strings.Index(serviceBody, "// CreateSandbox")
	if createToolStart < 0 || createToolEnd < createToolStart {
		t.Fatalf("CreateTool function block not found")
	}
	createToolBlock := serviceBody[createToolStart:createToolEnd]
	if !strings.Contains(createToolBlock, "parseToolResourceSpec(") {
		t.Fatalf("CreateTool must validate resource_spec before persistence")
	}
	if !strings.Contains(createToolBlock, "validateImageSecurityGate(") {
		t.Fatalf("CreateTool web-embed images must use the unified image security gate")
	}

	adminSrc, err := os.ReadFile("runtime_admin.go")
	if err != nil {
		t.Fatalf("read runtime_admin.go: %v", err)
	}
	adminBody := string(adminSrc)
	updateStart := strings.Index(adminBody, "func (s *Service) UpdateRuntime(")
	updateEnd := strings.Index(adminBody, "// GetRuntimeSelftest")
	if updateStart < 0 || updateEnd < updateStart {
		t.Fatalf("UpdateRuntime function block not found")
	}
	if !strings.Contains(adminBody[updateStart:updateEnd], "parseRuntimeAdapterSpec(spec)") {
		t.Fatalf("UpdateRuntime must validate adapter_spec before persistence")
	}
}

// TestUpdateRuntimeDoesNotPreserveStaleOptionalCapability 确认更新运行时按请求重写可选能力字段,不会继承旧 L2/L3 配置。
func TestUpdateRuntimeDoesNotPreserveStaleOptionalCapability(t *testing.T) {
	adminSrc, err := os.ReadFile("runtime_admin.go")
	if err != nil {
		t.Fatalf("read runtime_admin.go: %v", err)
	}
	adminBody := string(adminSrc)
	updateStart := strings.Index(adminBody, "func (s *Service) UpdateRuntime(")
	updateEnd := strings.Index(adminBody, "// GetRuntimeSelftest")
	if updateStart < 0 || updateEnd < updateStart {
		t.Fatalf("UpdateRuntime function block not found")
	}
	block := adminBody[updateStart:updateEnd]
	if strings.Contains(block, "current.CapabilityImpl.String") || strings.Contains(block, "current.PluginRef.String") {
		t.Fatalf("UpdateRuntime must not preserve stale capability_impl/plugin_ref when request omits them")
	}
	if !strings.Contains(block, "CapabilityImpl: pgText(req.CapabilityImpl)") ||
		!strings.Contains(block, "PluginRef:      pgText(req.PluginRef)") {
		t.Fatalf("UpdateRuntime must persist optional capability fields from the request")
	}
}

// TestImageSecurityGateRequiresConfiguredAttestation 确认镜像门禁只信任受控配置中的 CI/Harbor 证明。
func TestImageSecurityGateRequiresDigestSignatureAndScan(t *testing.T) {
	digest := "sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	imageURL := "harbor.chaimir.local/runtime/evm@" + digest
	cfg := config.SandboxConfig{
		ImageRegistry: "harbor.chaimir.local",
		ImageAttestations: []config.SandboxImageAttestation{{
			ImageURL:       imageURL,
			Digest:         digest,
			CosignVerified: true,
			TrivyStatus:    ImageScanPassed,
		}},
	}
	valid := ImageSecuritySpec{
		ImageURL: imageURL,
		Digest:   digest,
	}
	if err := validateImageSecurityGate(valid, cfg); err != nil {
		t.Fatalf("valid image security spec rejected: %v", err)
	}
	missingDigest := valid
	missingDigest.Digest = ""
	if err := validateImageSecurityGate(missingDigest, cfg); err == nil {
		t.Fatalf("image without digest must be rejected")
	}
	withoutAttestation := valid
	withoutAttestation.ImageURL = "harbor.chaimir.local/runtime/other@" + digest
	if err := validateImageSecurityGate(withoutAttestation, cfg); err == nil {
		t.Fatalf("image without configured attestation must be rejected")
	}
	unsignedCfg := cfg
	unsignedCfg.ImageAttestations = []config.SandboxImageAttestation{{
		ImageURL:    imageURL,
		Digest:      digest,
		TrivyStatus: ImageScanPassed,
	}}
	if err := validateImageSecurityGate(valid, unsignedCfg); err == nil {
		t.Fatalf("image with unsigned attestation must be rejected")
	}
	failedScanCfg := cfg
	failedScanCfg.ImageAttestations = []config.SandboxImageAttestation{{
		ImageURL:       imageURL,
		Digest:         digest,
		CosignVerified: true,
		TrivyStatus:    "failed",
	}}
	if err := validateImageSecurityGate(valid, failedScanCfg); err == nil {
		t.Fatalf("image with failed scan attestation must be rejected")
	}
}

// TestImageSecurityProofsAreNotRequestDTOFields 确认外部请求不能直接提交签名/扫描结论。
func TestImageSecurityProofsAreNotRequestDTOFields(t *testing.T) {
	src, err := os.ReadFile("dto.go")
	if err != nil {
		t.Fatalf("read dto.go: %v", err)
	}
	body := string(src)
	if strings.Contains(body, "CosignVerified") || strings.Contains(body, "TrivyStatus") {
		t.Fatalf("image signature and scan results must come from controlled attestation config, not request DTOs")
	}
}

// TestImageSecurityGateHasNoDisableSwitch 确认镜像安全门禁没有关闭开关。
func TestImageSecurityGateHasNoDisableSwitch(t *testing.T) {
	cfgSrc, err := os.ReadFile("../../platform/config/config.go")
	if err != nil {
		t.Fatalf("read config.go: %v", err)
	}
	if strings.Contains(string(cfgSrc), "ImageSecurityRequired") ||
		strings.Contains(string(cfgSrc), "SANDBOX_IMAGE_SECURITY_REQUIRED") {
		t.Fatalf("image security gate must not expose a disable switch")
	}
}

// TestRuntimeImageCreateCannotTrustRequestPrepulled 防止请求体绕过 DaemonSet Ready 闭环。
func TestRuntimeImageCreateCannotTrustRequestPrepulled(t *testing.T) {
	data, err := os.ReadFile("service.go")
	if err != nil {
		t.Fatalf("read service.go: %v", err)
	}
	body := string(data)
	start := strings.Index(body, "func (s *Service) CreateRuntimeImage(")
	end := strings.Index(body, "// ListTools")
	if start < 0 || end < start {
		t.Fatalf("CreateRuntimeImage function block not found")
	}
	block := body[start:end]
	if strings.Contains(block, "req.Prepulled") {
		t.Fatalf("CreateRuntimeImage must ignore request prepulled; only PrepullRuntimeImage may mark success")
	}
	if !strings.Contains(block, "validateImageSecurityGate(") {
		t.Fatalf("CreateRuntimeImage must use the unified image security gate")
	}
}

// TestCreateSandboxOnlySubmitsAsyncStartTask 确认创建请求返回前不直接执行 K8s 编排。
func TestCreateSandboxOnlySubmitsAsyncStartTask(t *testing.T) {
	data, err := os.ReadFile("service.go")
	if err != nil {
		t.Fatalf("read service.go: %v", err)
	}
	body := string(data)
	start := strings.Index(body, "func (s *Service) CreateSandbox(")
	end := strings.Index(body, "// startSandboxAsync")
	if start < 0 || end < start {
		t.Fatalf("CreateSandbox function block not found")
	}
	block := body[start:end]
	if strings.Contains(block, "s.orchestrator.Create(ctx, spec)") {
		t.Fatalf("CreateSandbox must not synchronously create K8s resources before returning phase=1")
	}

	asyncStart := strings.Index(body, "func (s *Service) startSandboxAsync(")
	asyncEnd := strings.Index(body, "// detachSandboxContext")
	if asyncStart < 0 || asyncEnd < asyncStart {
		t.Fatalf("startSandboxAsync function block not found")
	}
	asyncBlock := body[asyncStart:asyncEnd]
	if !strings.Contains(asyncBlock, "s.orchestrator.Create(ctx, spec)") {
		t.Fatalf("startSandboxAsync must own K8s Create before WaitReady")
	}
}

// TestStageTwoInitializationFailureDoesNotMarkSandboxError 确认阶段二失败不会伪装成 phase=4,也不会把可进入环境置 error。
func TestStageTwoInitializationFailureDoesNotMarkSandboxError(t *testing.T) {
	data, err := os.ReadFile("service.go")
	if err != nil {
		t.Fatalf("read service.go: %v", err)
	}
	body := string(data)
	start := strings.Index(body, "func (s *Service) startSandboxAsync(")
	end := strings.Index(body, "// detachSandboxContext")
	if start < 0 || end < start {
		t.Fatalf("startSandboxAsync function block not found")
	}
	block := body[start:end]
	if strings.Contains(block, `s.recordAsyncSandboxError(ctx, spec, "initialization", err)`) {
		t.Fatalf("stage two initialization failure must be recorded without marking sandbox error")
	}
	if !strings.Contains(block, "s.recordAsyncSandboxInitializationError") {
		t.Fatalf("stage two initialization failure must use a dedicated recorder")
	}
}

// TestChainCapabilityChecksSignedSourceRef 确认内部链能力不会只按 tenant+sandbox_id 越权访问其他来源沙箱。
func TestChainCapabilityChecksSignedSourceRef(t *testing.T) {
	data, err := os.ReadFile("service.go")
	if err != nil {
		t.Fatalf("read service.go: %v", err)
	}
	body := string(data)
	start := strings.Index(body, "func (s *Service) capabilityForSandbox(")
	end := strings.Index(body, "// markSandboxError")
	if start < 0 || end < start {
		t.Fatalf("capabilityForSandbox function block not found")
	}
	block := body[start:end]
	if !strings.Contains(block, "validateSandboxSourceRefAccess(ctx, row.SourceRef)") {
		t.Fatalf("capabilityForSandbox must verify signed source_ref before exposing chain capability")
	}
}

// TestRecycleBySourceRefDoesNotFinalizeOriginalAndRecycledRows 确认来源回收不会把同一沙箱加入 finalize 队列两次。
func TestRecycleBySourceRefDoesNotFinalizeOriginalAndRecycledRows(t *testing.T) {
	data, err := os.ReadFile("service.go")
	if err != nil {
		t.Fatalf("read service.go: %v", err)
	}
	body := string(data)
	start := strings.Index(body, "func (s *Service) RecycleBySourceRef(")
	end := strings.Index(body, "// DestroySandbox")
	if start < 0 || end < start {
		t.Fatalf("RecycleBySourceRef function block not found")
	}
	block := body[start:end]
	if strings.Contains(block, "rows = found") {
		t.Fatalf("RecycleBySourceRef must build finalize rows from locked recycling rows only")
	}
}

// TestSandboxRecycleRequiresEventBus 确认沙箱回收终态事件不能因总线缺失被静默跳过。
func TestSandboxRecycleRequiresEventBus(t *testing.T) {
	data, err := os.ReadFile("service.go")
	if err != nil {
		t.Fatalf("read service.go: %v", err)
	}
	body := string(data)
	if strings.Contains(body, "if s.bus != nil") {
		t.Fatalf("sandbox recycle must not skip sandbox.recycled when event bus is missing")
	}
	block := body[strings.Index(body, "func (s *Service) finalizeSandboxRecycle("):]
	if !strings.Contains(block, "SubjectSandboxRecycled") {
		t.Fatalf("finalizeSandboxRecycle must publish sandbox.recycled after destroy")
	}
}

// TestDestroySandboxChecksSignedSourceRefBeforeRecycle 确认内部按 id 销毁不能绕过 source_ref 归属。
func TestDestroySandboxChecksSignedSourceRefBeforeRecycle(t *testing.T) {
	data, err := os.ReadFile("service.go")
	if err != nil {
		t.Fatalf("read service.go: %v", err)
	}
	body := string(data)
	start := strings.Index(body, "func (s *Service) DestroySandbox(")
	end := strings.Index(body, "// finalizeSandboxRecycle")
	if start < 0 || end < start {
		t.Fatalf("DestroySandbox function block not found")
	}
	block := body[start:end]
	validateIdx := strings.Index(block, "authorizeSandboxRowAccess(ctx, id, current)")
	recycleIdx := strings.Index(block, "q.RecycleSandbox(ctx, sandboxID)")
	if validateIdx < 0 || recycleIdx < 0 || validateIdx > recycleIdx {
		t.Fatalf("DestroySandbox must validate signed source_ref before locking sandbox as recycling")
	}
}

// TestSandboxRecycleSchedulerUsesUnifiedRunnerAndExistingFinalizer 确认 M2 自动回收不新增第二套回收状态机。
func TestSandboxRecycleSchedulerUsesUnifiedRunnerAndExistingFinalizer(t *testing.T) {
	serviceSrc, err := os.ReadFile("service.go")
	if err != nil {
		t.Fatalf("read service.go: %v", err)
	}
	serviceBody := string(serviceSrc)
	for _, required := range []string{
		"StartRecycleScheduler",
		"RecycleDueSandboxesOnce",
		"background.Run",
		"ListDueSandboxRecycles",
		"ListExpiredSandboxSnapshots",
		"finalizeSandboxRecycle(ctx, row.TenantID, row, reason)",
	} {
		if !strings.Contains(serviceBody, required) {
			t.Fatalf("sandbox recycle scheduler missing %s", required)
		}
	}

	querySrc, err := os.ReadFile("../../../db/queries/sandbox.sql")
	if err != nil {
		t.Fatalf("read sandbox.sql: %v", err)
	}
	queryBody := string(querySrc)
	for _, required := range []string{
		"ListDueSandboxRecycles",
		"ListExpiredSandboxSnapshots",
		"tenant_quota",
		"FOR UPDATE SKIP LOCKED",
		"snapshot_expire_at <= now()",
	} {
		if !strings.Contains(queryBody, required) {
			t.Fatalf("sandbox recycle SQL missing %s", required)
		}
	}
}

// TestRuntimeImageURLMustUseConfiguredRegistry 确认运行时镜像只能来自配置声明的 Harbor/私有仓库前缀。
func TestRuntimeImageURLMustUseConfiguredRegistry(t *testing.T) {
	cfg := config.SandboxConfig{ImageRegistry: "harbor.chaimir.local"}
	if err := validateRuntimeImageURL("harbor.chaimir.local/runtime/evm:v1", cfg); err != nil {
		t.Fatalf("allowed registry image rejected: %v", err)
	}
	if err := validateRuntimeImageURL("docker.io/library/busybox:latest", cfg); err == nil {
		t.Fatalf("external registry image must be rejected")
	}
}

func validSandboxCreateRequest() contracts.SandboxCreateRequest {
	return contracts.SandboxCreateRequest{
		TenantID:       1001,
		RuntimeCode:    "evm-hardhat",
		ToolCodes:      []string{"terminal"},
		OwnerAccountID: 2001,
		SourceRef:      "experiment:2026:instance:55",
	}
}
