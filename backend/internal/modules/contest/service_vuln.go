// contest service_vuln 文件实现漏洞源配置、同步、预验证记录和内容固化。
package contest

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"chaimir/internal/contracts"
	"chaimir/internal/platform/audit"
	"chaimir/internal/platform/secretmap"
	"chaimir/pkg/apperr"
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
	if err := s.writeAudit(ctx, id.TenantID, id.AccountID, audit.ActorRoleTeacher, "contest.vuln_source.upsert", auditTargetVulnSource, item.ID, nil); err != nil {
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
	return out, s.writeAudit(ctx, id.TenantID, id.AccountID, audit.ActorRoleTeacher, "contest.vuln_source.sync", auditTargetVulnSource, source.ID, map[string]any{"count": len(out)})
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
	if err := s.writeAudit(ctx, id.TenantID, id.AccountID, audit.ActorRoleTeacher, "contest.vuln_problem.import", auditTargetVulnProblem, item.ID, nil); err != nil {
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

// SetVulnPrevalidate 保存漏洞题正反向预验证结果。
func (s *Service) SetVulnPrevalidate(ctx context.Context, problemID int64, req PrevalidateRequest) (VulnProblemDTO, error) {
	id, err := currentIdentity(ctx)
	if err != nil {
		return VulnProblemDTO{}, err
	}
	status := VulnPrevalidateFailed
	if req.Passed {
		status = VulnPrevalidatePassed
	}
	if req.Detail == nil {
		req.Detail = map[string]any{}
	}
	var item VulnProblem
	if err := s.store.TenantTx(ctx, id.TenantID, func(ctx context.Context, tx TxStore) error {
		var err error
		item, err = tx.SetVulnProblemPrevalidate(ctx, id.TenantID, problemID, status, req.Detail)
		return err
	}); err != nil {
		return VulnProblemDTO{}, err
	}
	return vulnProblemDTOFromModel(item), s.writeAudit(ctx, id.TenantID, id.AccountID, audit.ActorRoleTeacher, "contest.vuln_problem.prevalidate", auditTargetVulnProblem, item.ID, map[string]any{"passed": req.Passed})
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
	if err := s.writeAudit(ctx, id.TenantID, id.AccountID, audit.ActorRoleTeacher, "contest.vuln_problem.finalize", auditTargetVulnProblem, item.ID, map[string]any{"item_code": item.ContentItemCode, "item_version": item.ContentItemVersion}); err != nil {
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
		raw, err := json.Marshal(cfg["request_body"])
		if err != nil {
			return nil, apperr.ErrContestVulnSourceInvalid.WithCause(err)
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
	client := &http.Client{Timeout: time.Duration(timeout) * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, apperr.ErrContestVulnSourceFetchFailed.WithCause(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, apperr.ErrContestVulnSourceFetchFailed
	}
	limited := io.LimitReader(resp.Body, s.cfg.VulnSourceMaxResponseBytes+1)
	raw, err := io.ReadAll(limited)
	if err != nil {
		return nil, apperr.ErrContestVulnSourceFetchFailed.WithCause(err)
	}
	if int64(len(raw)) > s.cfg.VulnSourceMaxResponseBytes {
		return nil, apperr.ErrContestVulnSourceFetchFailed
	}
	var payload any
	if err := json.Unmarshal(raw, &payload); err != nil {
		return nil, apperr.ErrContestVulnSourceFetchFailed.WithCause(err)
	}
	nodes := selectCases(payload, stringFromMap(cfg, "cases_path"))
	out := make([]VulnProblem, 0, len(nodes))
	for _, node := range nodes {
		item, err := vulnProblemFromExternal(node, source.DefaultLevel)
		if err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	return out, nil
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
	u, err := url.Parse(endpoint)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
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
	for _, part := range strings.Split(strings.TrimSpace(path), ".") {
		if part == "" {
			continue
		}
		obj, ok := current.(map[string]any)
		if !ok {
			return nil
		}
		current = obj[part]
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
func vulnProblemFromExternal(item map[string]any, defaultLevel int16) (VulnProblem, error) {
	req := ImportVulnProblemRequest{ExternalRef: stringFromMap(item, "external_ref"), Title: stringFromMap(item, "title"), Level: int16FromAny(item["level"], defaultLevel), RuntimeMode: int16FromAny(item["runtime_mode"], VulnRuntimeIsolated), DraftBody: item}
	req, err := validateVulnProblemInput(req)
	if err != nil {
		return VulnProblem{}, err
	}
	return VulnProblem{ExternalRef: req.ExternalRef, Title: req.Title, Level: req.Level, RuntimeMode: req.RuntimeMode, DraftBody: req.DraftBody}, nil
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
