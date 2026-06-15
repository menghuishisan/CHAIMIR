// contest service_vuln 文件实现漏洞源配置、同步、预验证记录和内容固化。
package contest

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"chaimir/internal/contracts"
	"chaimir/internal/platform/jsonx"
	"chaimir/internal/platform/netx"
	"chaimir/internal/platform/secretmap"
	"chaimir/internal/platform/timex"
	"chaimir/pkg/apperr"
	"chaimir/pkg/chainassert"
	"chaimir/pkg/logging"
)

// UpsertVulnSource 创建或更新本租户漏洞源。
func (s *Service) UpsertVulnSource(ctx context.Context, req VulnSourceRequest) (VulnSourceDTO, error) {
	id, err := currentIdentity(ctx)
	if err != nil {
		return VulnSourceDTO{}, err
	}
	item, err := vulnSourceFromRequest(req, id.TenantID, s.ids.Generate())
	if err != nil {
		return VulnSourceDTO{}, err
	}
	if err := validateVulnSourceConfig(item.Config, s.cfg.VulnSourceTimeoutSeconds); err != nil {
		return VulnSourceDTO{}, err
	}
	protected, err := secretmap.Protect(s.cipher, item.Config, "漏洞源配置")
	if err != nil {
		return VulnSourceDTO{}, apperr.ErrContestVulnSourceInvalid.WithCause(err)
	}
	item.Config = protected
	if err := s.store.TenantTx(ctx, id.TenantID, func(ctx context.Context, tx TxStore) error {
		var err error
		item, err = tx.UpsertVulnSource(ctx, item)
		return err
	}); err != nil {
		return VulnSourceDTO{}, err
	}
	if err := s.writeAudit(ctx, id.TenantID, id.AccountID, contracts.RoleNumTeacher, "contest.vuln_source.upsert", auditTargetVulnSource, item.ID, nil); err != nil {
		return VulnSourceDTO{}, err
	}
	return vulnSourceDTOFromModel(item), nil
}

// ListVulnSources 查询平台预置源和本租户源。
func (s *Service) ListVulnSources(ctx context.Context) ([]VulnSourceDTO, error) {
	id, err := currentIdentity(ctx)
	if err != nil {
		return nil, err
	}
	var items []VulnSource
	if err := s.store.TenantTx(ctx, id.TenantID, func(ctx context.Context, tx TxStore) error {
		var err error
		items, err = tx.ListVulnSources(ctx, id.TenantID)
		return err
	}); err != nil {
		return nil, err
	}
	out := make([]VulnSourceDTO, 0, len(items))
	for _, item := range items {
		out = append(out, vulnSourceDTOFromModel(item))
	}
	return out, nil
}

// SyncVulnSource 拉取外部漏洞源并写入漏洞题草稿。
func (s *Service) SyncVulnSource(ctx context.Context, sourceID int64) ([]VulnProblemDTO, error) {
	id, err := currentIdentity(ctx)
	if err != nil {
		return nil, err
	}
	var source VulnSource
	if err := s.store.TenantTx(ctx, id.TenantID, func(ctx context.Context, tx TxStore) error {
		var err error
		source, err = tx.GetVulnSource(ctx, id.TenantID, sourceID)
		return err
	}); err != nil {
		return nil, err
	}
	if !source.Enabled {
		return nil, apperr.ErrContestVulnSourceInvalid
	}
	revealed, err := secretmap.Reveal(s.cipher, source.Config, "漏洞源配置")
	if err != nil {
		return nil, apperr.ErrContestVulnSourceInvalid.WithCause(err)
	}
	source.Config = revealed
	cases, err := s.fetchVulnCases(ctx, source)
	if err != nil {
		return nil, err
	}
	var problems []VulnProblem
	if err := s.store.TenantTx(ctx, id.TenantID, func(ctx context.Context, tx TxStore) error {
		for _, item := range cases {
			if item.Level == 0 {
				item.Level = source.DefaultLevel
			}
			item.ID = s.ids.Generate()
			item.TenantID = id.TenantID
			item.SourceID = source.ID
			item, err = tx.UpsertVulnProblem(ctx, item)
			if err != nil {
				return err
			}
			problems = append(problems, item)
		}
		_, err := tx.MarkVulnSourceSynced(ctx, id.TenantID, source.ID)
		return err
	}); err != nil {
		return nil, err
	}
	out := make([]VulnProblemDTO, 0, len(problems))
	for _, item := range problems {
		out = append(out, vulnProblemDTOFromModel(item))
	}
	return out, s.writeAudit(ctx, id.TenantID, id.AccountID, contracts.RoleNumTeacher, "contest.vuln_source.sync", auditTargetVulnSource, source.ID, map[string]any{"count": len(out)})
}

// ImportVulnProblem 手动导入一个漏洞案例草稿。
func (s *Service) ImportVulnProblem(ctx context.Context, req ImportVulnProblemRequest) (VulnProblemDTO, error) {
	id, err := currentIdentity(ctx)
	if err != nil {
		return VulnProblemDTO{}, err
	}
	req, err = validateVulnProblemInput(req)
	if err != nil {
		return VulnProblemDTO{}, err
	}
	item := VulnProblem{ID: s.ids.Generate(), TenantID: id.TenantID, SourceID: req.SourceID, ExternalRef: req.ExternalRef, Title: req.Title, Level: req.Level, RuntimeMode: req.RuntimeMode, DraftBody: req.DraftBody}
	if err := s.store.TenantTx(ctx, id.TenantID, func(ctx context.Context, tx TxStore) error {
		if req.SourceID > 0 {
			if _, err := tx.GetVulnSource(ctx, id.TenantID, req.SourceID); err != nil {
				return err
			}
		}
		var err error
		item, err = tx.UpsertVulnProblem(ctx, item)
		return err
	}); err != nil {
		return VulnProblemDTO{}, err
	}
	if err := s.writeAudit(ctx, id.TenantID, id.AccountID, contracts.RoleNumTeacher, "contest.vuln_problem.import", auditTargetVulnProblem, item.ID, nil); err != nil {
		return VulnProblemDTO{}, err
	}
	return vulnProblemDTOFromModel(item), nil
}

// ListVulnProblems 查询漏洞题草稿列表。
func (s *Service) ListVulnProblems(ctx context.Context, sourceID int64, status int16, page, size int) ([]VulnProblemDTO, error) {
	id, err := currentIdentity(ctx)
	if err != nil {
		return nil, err
	}
	var items []VulnProblem
	if err := s.store.TenantTx(ctx, id.TenantID, func(ctx context.Context, tx TxStore) error {
		var err error
		items, err = tx.ListVulnProblems(ctx, id.TenantID, sourceID, status, page, size)
		return err
	}); err != nil {
		return nil, err
	}
	out := make([]VulnProblemDTO, 0, len(items))
	for _, item := range items {
		out = append(out, vulnProblemDTOFromModel(item))
	}
	return out, nil
}

// SetVulnPrevalidate 运行漏洞题正反向预验证并保存结果。
func (s *Service) SetVulnPrevalidate(ctx context.Context, problemID int64, req PrevalidateRequest) (VulnProblemDTO, error) {
	id, err := currentIdentity(ctx)
	if err != nil {
		return VulnProblemDTO{}, err
	}
	req, err = validatePrevalidateRequest(req)
	if err != nil {
		return VulnProblemDTO{}, err
	}
	var current VulnProblem
	if err := s.store.TenantTx(ctx, id.TenantID, func(ctx context.Context, tx TxStore) error {
		var err error
		current, err = tx.GetVulnProblem(ctx, id.TenantID, problemID)
		return err
	}); err != nil {
		return VulnProblemDTO{}, err
	}
	status, detail := s.runVulnPrevalidation(ctx, id.TenantID, id.AccountID, current, req)
	var item VulnProblem
	if err := s.store.TenantTx(ctx, id.TenantID, func(ctx context.Context, tx TxStore) error {
		var err error
		item, err = tx.SetVulnProblemPrevalidate(ctx, id.TenantID, problemID, status, detail)
		return err
	}); err != nil {
		return VulnProblemDTO{}, err
	}
	return vulnProblemDTOFromModel(item), s.writeAudit(ctx, id.TenantID, id.AccountID, contracts.RoleNumTeacher, "contest.vuln_problem.prevalidate", auditTargetVulnProblem, item.ID, map[string]any{"status": status})
}

// FinalizeVulnProblem 把预验证通过的漏洞题固化到 M5 内容中心。
func (s *Service) FinalizeVulnProblem(ctx context.Context, problemID int64) (VulnProblemDTO, error) {
	id, err := currentIdentity(ctx)
	if err != nil {
		return VulnProblemDTO{}, err
	}
	var item VulnProblem
	if err := s.store.TenantTx(ctx, id.TenantID, func(ctx context.Context, tx TxStore) error {
		var err error
		item, err = tx.GetVulnProblem(ctx, id.TenantID, problemID)
		return err
	}); err != nil {
		return VulnProblemDTO{}, err
	}
	if item.PrevalidateStatus != VulnPrevalidatePassed {
		return VulnProblemDTO{}, apperr.ErrContestVulnPrevalidateFailed
	}
	snapshot, err := s.contentImport.SystemImportContent(ctx, contracts.ContentSystemImportRequest{TenantID: id.TenantID, Code: stableContestCode(item), Version: "v1", Type: contentTypeContestProblem, Title: item.Title, Difficulty: contentDifficultyBasic, AuthorID: id.AccountID, AuthorType: contentAuthorExternal, Visibility: contentVisibilityTenant, Body: item.DraftBody, SensitiveFields: []string{"answer", "flag", "judge"}, AutoPublish: true, SystemImportNote: map[string]any{"source": "contest_vuln_problem", "vuln_problem_id": item.ID}})
	if err != nil {
		return VulnProblemDTO{}, apperr.ErrContestVulnFinalizeFailed.WithCause(err)
	}
	if err := s.store.TenantTx(ctx, id.TenantID, func(ctx context.Context, tx TxStore) error {
		var err error
		item, err = tx.FinalizeVulnProblem(ctx, id.TenantID, problemID, snapshot.ItemCode, snapshot.ItemVersion)
		return err
	}); err != nil {
		return VulnProblemDTO{}, err
	}
	if err := s.writeAudit(ctx, id.TenantID, id.AccountID, contracts.RoleNumTeacher, "contest.vuln_problem.finalize", auditTargetVulnProblem, item.ID, map[string]any{"item_code": item.ContentItemCode, "item_version": item.ContentItemVersion}); err != nil {
		return VulnProblemDTO{}, err
	}
	return vulnProblemDTOFromModel(item), nil
}

// fetchVulnCases 根据源配置拉取并解析漏洞案例。
func (s *Service) fetchVulnCases(ctx context.Context, source VulnSource) ([]VulnProblem, error) {
	cfg := source.Config
	endpoint, _ := cfg["endpoint"].(string)
	method, _ := cfg["method"].(string)
	if method == "" {
		method = http.MethodGet
	}
	method = strings.ToUpper(strings.TrimSpace(method))
	timeout := sourceTimeoutSeconds(cfg, s.cfg.VulnSourceTimeoutSeconds)
	var body io.Reader
	if method == http.MethodPost {
		raw, err := jsonx.AnyBytes(cfg["body"], apperr.ErrContestVulnSourceInvalid)
		if err != nil {
			return nil, err
		}
		body = bytes.NewReader(raw)
	}
	req, err := http.NewRequestWithContext(ctx, method, endpoint, body)
	if err != nil {
		return nil, apperr.ErrContestVulnSourceInvalid.WithCause(err)
	}
	req.Header.Set("Accept", "application/json")
	if method == http.MethodPost {
		req.Header.Set("Content-Type", "application/json")
	}
	for key, value := range stringMapFromAny(cfg["headers"]) {
		if key != "" && value != "" {
			req.Header.Set(key, value)
		}
	}
	client, err := netx.NewPublicHTTPClient(time.Duration(timeout) * time.Second)
	if err != nil {
		return nil, apperr.ErrContestVulnSourceInvalid.WithCause(err)
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, apperr.ErrContestVulnSourceFetchFailed.WithCause(err)
	}
	defer logging.CloseContext(ctx, "关闭漏洞源响应失败", resp.Body)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, apperr.ErrContestVulnSourceBadStatus
	}
	limited := io.LimitReader(resp.Body, s.cfg.VulnSourceMaxResponseBytes+1)
	raw, err := io.ReadAll(limited)
	if err != nil {
		return nil, apperr.ErrContestVulnSourceReadFailed.WithCause(err)
	}
	if int64(len(raw)) > s.cfg.VulnSourceMaxResponseBytes {
		return nil, apperr.ErrContestVulnSourceTooLarge
	}
	var payload any
	if err := jsonx.DecodeStrict(raw, &payload); err != nil {
		return nil, apperr.ErrContestVulnSourceJSONInvalid.WithCause(err)
	}
	nodes := selectCases(payload, stringFromMap(cfg, "cases_path"))
	mapping := stringMapFromAny(cfg["mapping"])
	out := make([]VulnProblem, 0, len(nodes))
	for _, node := range nodes {
		item, err := vulnProblemFromExternal(node, mapping, source.DefaultLevel)
		if err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	return out, nil
}

// runVulnPrevalidation 在隔离沙箱中执行正向 PoC 与反向不误判验证。
func (s *Service) runVulnPrevalidation(ctx context.Context, tenantID, accountID int64, problem VulnProblem, req PrevalidateRequest) (int16, map[string]any) {
	detail := map[string]any{"positive": map[string]any{}, "negative": map[string]any{}}
	positive, err := s.runVulnValidationCase(ctx, tenantID, accountID, problem, req, "positive", true)
	detail["positive"] = positive
	if err != nil {
		detail["error"] = safeDetailError(err)
		return VulnPrevalidateFailed, detail
	}
	negative, err := s.runVulnValidationCase(ctx, tenantID, accountID, problem, req, "negative", false)
	detail["negative"] = negative
	if err != nil {
		detail["error"] = safeDetailError(err)
		return VulnPrevalidateFailed, detail
	}
	if !boolFromMap(positive, "passed") || !boolFromMap(negative, "passed") {
		return VulnPrevalidateFailed, detail
	}
	return VulnPrevalidatePassed, detail
}

// runVulnValidationCase 执行一条正向或反向预验证用例。
func (s *Service) runVulnValidationCase(ctx context.Context, tenantID, accountID int64, problem VulnProblem, req PrevalidateRequest, phase string, positive bool) (result map[string]any, retErr error) {
	sourceRef := fmt.Sprintf("contest:%04d:vuln-prevalidate:%d:%s", timex.Now().Year(), problem.ID, phase)
	info, err := s.sandbox.CreateSandbox(ctx, contracts.SandboxCreateRequest{TenantID: tenantID, RuntimeCode: req.RuntimeCode, RuntimeImageVersion: req.RuntimeImageVersion, ToolCodes: req.ToolCodes, InitCodeRef: req.InitCodeRef, InitScriptRef: req.InitScriptRef, OwnerAccountID: accountID, SourceRef: sourceRef, KeepAlive: false, SnapshotEnabled: true})
	if err != nil {
		return nil, apperr.ErrContestSandboxUnavailable.WithCause(err)
	}
	defer func() {
		if recycleErr := s.sandbox.RecycleBySourceRef(ctx, contracts.SandboxRecycleRequest{TenantID: tenantID, SourceRef: sourceRef, Reason: "vuln_prevalidate"}); recycleErr != nil {
			if retErr != nil {
				retErr = apperr.ErrContestSandboxUnavailable.WithCause(fmt.Errorf("漏洞预验证失败: %w; 回收沙箱失败: %v", retErr, recycleErr))
				return
			}
			retErr = apperr.ErrContestSandboxUnavailable.WithCause(recycleErr)
		}
	}()
	for _, step := range validationSteps(problem.DraftBody, "init_steps") {
		if err := s.runVulnChainStep(ctx, tenantID, info.SandboxID, sourceRef, step); err != nil {
			return nil, err
		}
	}
	if positive {
		for _, step := range validationSteps(problem.DraftBody, "positive_steps") {
			if err := s.runVulnChainStep(ctx, tenantID, info.SandboxID, sourceRef, step); err != nil {
				return nil, err
			}
		}
	}
	results, err := s.checkVulnAssertions(ctx, tenantID, info.SandboxID, sourceRef, validationSteps(problem.DraftBody, "assertions"), positive)
	if err != nil {
		return nil, err
	}
	return map[string]any{"passed": allAssertionResults(results), "assertions": results}, nil
}

// runVulnChainStep 调用 M2 链能力执行预验证步骤。
func (s *Service) runVulnChainStep(ctx context.Context, tenantID, sandboxID int64, sourceRef string, step map[string]any) error {
	switch strings.ToLower(stringFromAny(step["op"])) {
	case "deploy":
		_, err := s.sandbox.ChainDeploy(ctx, contracts.SandboxChainDeployRequest{TenantID: tenantID, SandboxID: sandboxID, SourceRef: sourceRef, Payload: mapAny(step["payload"])})
		return err
	case "tx":
		_, err := s.sandbox.ChainSendTx(ctx, contracts.SandboxChainTxRequest{TenantID: tenantID, SandboxID: sandboxID, SourceRef: sourceRef, Payload: mapAny(step["payload"])})
		return err
	case "reset":
		return s.sandbox.ChainReset(ctx, contracts.SandboxChainResetRequest{TenantID: tenantID, SandboxID: sandboxID, SourceRef: sourceRef})
	case "query", "":
		return nil
	default:
		return apperr.ErrContestVulnProblemInvalid
	}
}

// checkVulnAssertions 检查正向应通过、反向应全部不通过的断言集合。
func (s *Service) checkVulnAssertions(ctx context.Context, tenantID, sandboxID int64, sourceRef string, assertions []map[string]any, positive bool) ([]map[string]any, error) {
	if len(assertions) == 0 {
		return nil, apperr.ErrContestVulnProblemInvalid
	}
	out := make([]map[string]any, 0, len(assertions))
	for _, raw := range assertions {
		assertion := chainassert.FromMap(raw)
		actual, err := s.sandbox.ChainQuery(ctx, contracts.SandboxChainQueryRequest{TenantID: tenantID, SandboxID: sandboxID, SourceRef: sourceRef, Target: assertion.Target})
		if err != nil {
			return nil, apperr.ErrContestSandboxUnavailable.WithCause(err)
		}
		result := chainassert.Check(assertion, actual)
		passed := result.Passed
		if !positive {
			passed = !result.Passed
		}
		out = append(out, map[string]any{"case": result.Case, "passed": passed, "expected_label": result.ExpectedLabel, "actual": result.Actual, "hint": result.Hint})
	}
	return out, nil
}

// validationSteps 从漏洞草稿读取链步骤或断言数组。
func validationSteps(body map[string]any, key string) []map[string]any {
	raw, ok := body[key].([]any)
	if !ok {
		return nil
	}
	out := make([]map[string]any, 0, len(raw))
	for _, item := range raw {
		if m := mapAny(item); len(m) > 0 {
			out = append(out, m)
		}
	}
	return out
}

// allAssertionResults 判断断言结果是否全部通过。
func allAssertionResults(items []map[string]any) bool {
	if len(items) == 0 {
		return false
	}
	for _, item := range items {
		if !boolFromMap(item, "passed") {
			return false
		}
	}
	return true
}

// boolFromMap 从 map 中读取布尔字段。
func boolFromMap(m map[string]any, key string) bool {
	v, _ := m[key].(bool)
	return v
}

// safeDetailError 返回可写入预验证详情的脱敏错误摘要。
func safeDetailError(err error) string {
	if err == nil {
		return ""
	}
	msg := logging.SanitizeError(err.Error())
	if len(msg) > 256 {
		return msg[:256]
	}
	return msg
}

// vulnSourceFromRequest 归一化漏洞源请求。
func vulnSourceFromRequest(req VulnSourceRequest, tenantID, generatedID int64) (VulnSource, error) {
	req.Name = strings.TrimSpace(req.Name)
	if req.ID <= 0 {
		req.ID = generatedID
	}
	if req.Type <= 0 || req.Name == "" || len(req.Name) > 128 || req.Config == nil || (req.DefaultLevel != VulnLevelA && req.DefaultLevel != VulnLevelB && req.DefaultLevel != VulnLevelC) {
		return VulnSource{}, apperr.ErrContestVulnSourceInvalid
	}
	return VulnSource{ID: req.ID, TenantID: tenantID, Type: req.Type, Name: req.Name, Config: req.Config, DefaultLevel: req.DefaultLevel, Enabled: req.Enabled}, nil
}

// validateVulnSourceConfig 校验 HTTP 源配置边界。
func validateVulnSourceConfig(cfg map[string]any, defaultTimeout int) error {
	endpoint := stringFromMap(cfg, "endpoint")
	if _, err := netx.ValidatePublicHTTPURL(endpoint); err != nil {
		return apperr.ErrContestVulnSourceInvalid
	}
	method := strings.ToUpper(stringFromMap(cfg, "method"))
	if method == "" {
		method = http.MethodGet
	}
	if method != http.MethodGet && method != http.MethodPost {
		return apperr.ErrContestVulnSourceInvalid
	}
	timeout := sourceTimeoutSeconds(cfg, defaultTimeout)
	if timeout < 1 || timeout > 60 {
		return apperr.ErrContestVulnSourceInvalid
	}
	mapping := stringMapFromAny(cfg["mapping"])
	for _, key := range []string{"external_ref", "title", "draft_body"} {
		if strings.TrimSpace(mapping[key]) == "" {
			return apperr.ErrContestVulnSourceInvalid
		}
	}
	return nil
}

// sourceTimeoutSeconds 读取源级超时,缺失时使用启动配置。
func sourceTimeoutSeconds(cfg map[string]any, defaultTimeout int) int {
	switch v := cfg["timeout_seconds"].(type) {
	case float64:
		return int(v)
	case int:
		return v
	default:
		return defaultTimeout
	}
}

// selectCases 从 JSON 载荷中选择案例数组。
func selectCases(payload any, path string) []map[string]any {
	current := payload
	if root, ok := payload.(map[string]any); ok && strings.TrimSpace(path) != "" {
		current = jsonx.ValueFromPath(root, path)
	}
	switch v := current.(type) {
	case []any:
		out := make([]map[string]any, 0, len(v))
		for _, item := range v {
			if obj, ok := item.(map[string]any); ok {
				out = append(out, obj)
			}
		}
		return out
	case map[string]any:
		return []map[string]any{v}
	default:
		return nil
	}
}

// vulnProblemFromExternal 将外部漏洞案例映射为平台草稿。
func vulnProblemFromExternal(item map[string]any, mapping map[string]string, defaultLevel int16) (VulnProblem, error) {
	body := jsonx.ObjectFromAny(jsonx.ValueFromPath(item, mapping["draft_body"]))
	req := ImportVulnProblemRequest{
		ExternalRef: strings.TrimSpace(jsonx.StringFromPath(item, mapping["external_ref"])),
		Title:       strings.TrimSpace(jsonx.StringFromPath(item, mapping["title"])),
		Level:       vulnLevelFromAny(jsonx.ValueFromPath(item, mapping["level"]), defaultLevel),
		RuntimeMode: vulnRuntimeFromAny(jsonx.ValueFromPath(item, mapping["runtime_mode"]), VulnRuntimeIsolated),
		DraftBody:   body,
	}
	req, err := validateVulnProblemInput(req)
	if err != nil {
		return VulnProblem{}, err
	}
	return VulnProblem{ExternalRef: req.ExternalRef, Title: req.Title, Level: req.Level, RuntimeMode: req.RuntimeMode, DraftBody: req.DraftBody}, nil
}

// stringMapFromAny 将 JSON map 转为字符串映射,非法项按配置错误处理前保留为空。
func stringMapFromAny(v any) map[string]string {
	return jsonx.StringMapFromAny(v)
}

// mapAny 读取 JSON 对象。
func mapAny(v any) map[string]any {
	return jsonx.ObjectFromAny(v)
}

// stringFromAny 读取字符串值。
func stringFromAny(v any) string {
	return strings.TrimSpace(jsonx.StringFromAny(v))
}

// vulnLevelFromAny 解析 A/B/C 或 1/2/3 分级。
func vulnLevelFromAny(v any, defaultValue int16) int16 {
	switch x := v.(type) {
	case string:
		text := strings.ToUpper(strings.TrimSpace(x))
		if n, err := strconv.ParseInt(text, 10, 16); err == nil {
			return int16(n)
		}
		switch text {
		case "A":
			return VulnLevelA
		case "B":
			return VulnLevelB
		case "C":
			return VulnLevelC
		}
	case float64:
		return int16(x)
	case int:
		return int16(x)
	case int16:
		return x
	}
	return defaultValue
}

// vulnRuntimeFromAny 解析 isolated/forked 或 1/2 运行时。
func vulnRuntimeFromAny(v any, defaultValue int16) int16 {
	switch x := v.(type) {
	case string:
		text := strings.ToLower(strings.TrimSpace(x))
		if n, err := strconv.ParseInt(text, 10, 16); err == nil {
			return int16(n)
		}
		switch text {
		case "isolated":
			return VulnRuntimeIsolated
		case "forked":
			return VulnRuntimeForked
		}
	case float64:
		return int16(x)
	case int:
		return int16(x)
	case int16:
		return x
	}
	return defaultValue
}

// stringFromMap 安全读取 JSON map 中的字符串字段。
func stringFromMap(m map[string]any, key string) string {
	if v, ok := m[key].(string); ok {
		return strings.TrimSpace(v)
	}
	return ""
}

// int16FromAny 把 JSON 数字转换为枚举值。
func int16FromAny(v any, defaultValue int16) int16 {
	switch n := v.(type) {
	case float64:
		return int16(n)
	case int:
		return int16(n)
	default:
		return defaultValue
	}
}
