// contest service_vuln 文件实现漏洞源配置、同步、预验证记录和内容固化。
package contest

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"chaimir/internal/contracts"
	"chaimir/internal/platform/auth"
	"chaimir/internal/platform/ids"
	"chaimir/internal/platform/jsonx"
	"chaimir/internal/platform/netx"
	"chaimir/internal/platform/pagex"

	"chaimir/internal/platform/secretmap"
	"chaimir/internal/platform/tenant"
	"chaimir/internal/platform/timex"
	"chaimir/internal/platform/upload"
	"chaimir/pkg/apperr"
	"chaimir/pkg/chainassert"
	"chaimir/pkg/logging"
)

// UpsertPlatformVulnSource 创建或更新平台全局漏洞源，不产生租户漏洞题草稿。
func (s *Service) UpsertPlatformVulnSource(ctx context.Context, req VulnSourceRequest) (VulnSourceDTO, error) {
	id, ok := tenant.FromContext(ctx)
	if !ok || !id.IsPlatform || id.AccountID <= 0 {
		return VulnSourceDTO{}, apperr.ErrUnauthorized
	}
	item, err := vulnSourceFromRequest(req, 0, s.ids.Generate())
	if err != nil {
		return VulnSourceDTO{}, err
	}
	if err := s.store.PlatformTx(ctx, func(ctx context.Context, tx TxStore) error {
		var err error
		item, err = s.saveVulnSource(ctx, tx, item, req.ID > 0)
		if err != nil {
			return err
		}
		return s.writeAudit(ctx, 0, id.AccountID, contracts.RoleNumPlatformAdmin, "contest.vuln_source.upsert", auditTargetVulnSource, item.ID, nil)
	}); err != nil {
		return VulnSourceDTO{}, err
	}
	return vulnSourceDTOFromModel(item), nil
}

// ListPlatformVulnSources 查询平台全局漏洞源，不返回学校自建配置。
func (s *Service) ListPlatformVulnSources(ctx context.Context) ([]VulnSourceDTO, error) {
	id, ok := tenant.FromContext(ctx)
	if !ok || !id.IsPlatform || id.AccountID <= 0 {
		return nil, apperr.ErrUnauthorized
	}
	var items []VulnSource
	if err := s.store.PlatformTx(ctx, func(ctx context.Context, tx TxStore) error {
		var err error
		items, err = tx.ListVulnSources(ctx, 0)
		return err
	}); err != nil {
		return nil, err
	}
	out := make([]VulnSourceDTO, 0, len(items))
	for _, item := range items {
		if item.TenantID == 0 {
			out = append(out, vulnSourceDTOFromModel(item))
		}
	}
	return out, nil
}

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
	if err := s.store.TenantTx(ctx, id.TenantID, func(ctx context.Context, tx TxStore) error {
		var err error
		item, err = s.saveVulnSource(ctx, tx, item, req.ID > 0)
		if err != nil {
			return err
		}
		return s.writeAudit(ctx, id.TenantID, id.AccountID, contracts.RoleNumTeacher, "contest.vuln_source.upsert", auditTargetVulnSource, item.ID, nil)
	}); err != nil {
		return VulnSourceDTO{}, err
	}
	return vulnSourceDTOFromModel(item), nil
}

func (s *Service) saveVulnSource(ctx context.Context, tx TxStore, item VulnSource, editing bool) (VulnSource, error) {
	if _, err := parseVulnSourceConfig(item.Config, s.cfg.VulnSourceTimeoutSeconds); err != nil {
		return VulnSource{}, err
	}
	var stored map[string]any
	if editing {
		current, err := tx.LockVulnSource(ctx, item.TenantID, item.ID)
		if err != nil {
			return VulnSource{}, err
		}
		stored = current.Config
	}
	protected, err := secretmap.Protect(s.cipher, item.Config, stored, "漏洞源配置")
	if err != nil {
		return VulnSource{}, apperr.ErrContestVulnSourceInvalid.WithCause(err)
	}
	item.Config = protected
	return tx.UpsertVulnSource(ctx, item)
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
			item.ID = s.ids.Generate()
			item.TenantID = id.TenantID
			item.SourceID = source.ID
			item, err = tx.UpsertVulnProblem(ctx, item)
			if isNoRows(err) {
				continue // 非草稿冲突由 SQL 拒绝，不能因来源同步重新打开。
			}
			if err != nil {
				return err
			}
			problems = append(problems, item)
		}
		if source.TenantID == id.TenantID {
			if _, err := tx.MarkVulnSourceSynced(ctx, id.TenantID, source.ID); err != nil {
				return err
			}
		}
		return s.writeAudit(ctx, id.TenantID, id.AccountID, contracts.RoleNumTeacher, "contest.vuln_source.sync", auditTargetVulnSource, source.ID, map[string]any{"count": len(problems)})
	}); err != nil {
		return nil, err
	}
	out := make([]VulnProblemDTO, 0, len(problems))
	for _, item := range problems {
		out = append(out, vulnProblemDTOFromModel(item))
	}
	return out, nil
}

// CreateVulnProblem 新建漏洞题草稿，不覆盖已有案例。
func (s *Service) CreateVulnProblem(ctx context.Context, req VulnProblemRequest) (VulnProblemDTO, error) {
	id, err := currentIdentity(ctx)
	if err != nil {
		return VulnProblemDTO{}, err
	}
	req, err = validateVulnProblemInput(req)
	if err != nil {
		return VulnProblemDTO{}, err
	}
	item := VulnProblem{ID: s.ids.Generate(), TenantID: id.TenantID, SourceID: req.SourceID.Int64(), ExternalRef: req.ExternalRef, Title: req.Title, Level: req.Level, RuntimeMode: req.RuntimeMode, DraftBody: req.DraftBody}
	if err := s.store.TenantTx(ctx, id.TenantID, func(ctx context.Context, tx TxStore) error {
		if req.SourceID > 0 {
			if _, err := tx.GetVulnSource(ctx, id.TenantID, req.SourceID.Int64()); err != nil {
				return err
			}
		}
		var err error
		item, err = tx.CreateVulnProblem(ctx, item)
		if err != nil {
			return err
		}
		return s.writeAudit(ctx, id.TenantID, id.AccountID, contracts.RoleNumTeacher, "contest.vuln_problem.create", auditTargetVulnProblem, item.ID, nil)
	}); err != nil {
		return VulnProblemDTO{}, err
	}
	return vulnProblemDTOFromModel(item), nil
}

// GetVulnProblem 读取当前学校的完整草稿，供恢复编辑和展示验证结果。
func (s *Service) GetVulnProblem(ctx context.Context, problemID int64) (VulnProblemDTO, error) {
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
	return vulnProblemDTOFromModel(item), nil
}

// UpdateVulnProblem 只编辑调用方读过的草稿版本，写入与审计同事务提交。
func (s *Service) UpdateVulnProblem(ctx context.Context, problemID int64, req UpdateVulnProblemRequest) (VulnProblemDTO, error) {
	id, err := currentIdentity(ctx)
	if err != nil {
		return VulnProblemDTO{}, err
	}
	if req.ExpectedUpdatedAt.IsZero() {
		return VulnProblemDTO{}, apperr.ErrContestVulnProblemInvalid
	}
	input, err := validateVulnProblemInput(req.VulnProblemRequest)
	if err != nil {
		return VulnProblemDTO{}, err
	}
	item := VulnProblem{ID: problemID, TenantID: id.TenantID, SourceID: input.SourceID.Int64(), ExternalRef: input.ExternalRef, Title: input.Title, Level: input.Level, RuntimeMode: input.RuntimeMode, DraftBody: input.DraftBody, UpdatedAt: req.ExpectedUpdatedAt}
	if err := s.store.TenantTx(ctx, id.TenantID, func(ctx context.Context, tx TxStore) error {
		if item.SourceID > 0 {
			if _, err := tx.GetVulnSource(ctx, id.TenantID, item.SourceID); err != nil {
				return err
			}
		}
		var err error
		item, err = tx.UpdateVulnProblem(ctx, item)
		if err != nil {
			return err
		}
		return s.writeAudit(ctx, id.TenantID, id.AccountID, contracts.RoleNumTeacher, "contest.vuln_problem.update", auditTargetVulnProblem, item.ID, nil)
	}); err != nil {
		return VulnProblemDTO{}, err
	}
	return vulnProblemDTOFromModel(item), nil
}

// ListVulnProblems 查询漏洞题草稿分页列表。
func (s *Service) ListVulnProblems(ctx context.Context, sourceID int64, status, prevalidateStatus int16, page, size int) ([]VulnProblemDTO, int64, int, int, pagex.Facets, error) {
	id, err := currentIdentity(ctx)
	if err != nil {
		return nil, 0, page, size, nil, err
	}
	page, size = pagex.Normalize(page, size)
	var items []VulnProblem
	var total int64
	var facetRows []vulnFacetRow
	if err := s.store.TenantTx(ctx, id.TenantID, func(ctx context.Context, tx TxStore) error {
		var err error
		items, total, err = tx.ListVulnProblems(ctx, id.TenantID, sourceID, status, prevalidateStatus, page, size)
		if err != nil {
			return err
		}
		facetRows, err = tx.CountVulnProblemsByStatus(ctx, id.TenantID, sourceID)
		return err
	}); err != nil {
		return nil, 0, page, size, nil, err
	}
	out := make([]VulnProblemDTO, 0, len(items))
	for _, item := range items {
		out = append(out, vulnProblemDTOFromModel(item))
	}
	return out, total, page, size, vulnFacets(facetRows, status, prevalidateStatus), nil
}

// SetVulnPrevalidate 运行漏洞题正反向预验证并保存结果。
func (s *Service) SetVulnPrevalidate(ctx context.Context, problemID int64) (VulnProblemDTO, error) {
	id, err := currentIdentity(ctx)
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
	if current.Status != VulnProblemStatusDraft {
		return VulnProblemDTO{}, apperr.ErrContestVulnPrevalidateFailed
	}
	spec, err := vulnPrevalidateSpec(current.DraftBody)
	if err != nil {
		return VulnProblemDTO{}, err
	}
	if current.RuntimeMode == VulnRuntimeForked {
		if err := bindVulnForkIdentity(&spec, current.DraftBody); err != nil {
			return VulnProblemDTO{}, err
		}
	}
	spec.ID = fmt.Sprintf("contest:%04d:vuln:%s", timex.Now().Year(), ids.Format(problemID))
	snapshot, err := s.sandbox.CompileSandboxComposition(ctx, id.TenantID, spec)
	if err != nil {
		return VulnProblemDTO{}, apperr.ErrContestSandboxUnavailable.WithCause(err)
	}
	current.PrevalidateStatus, current.PrevalidateDetail = s.runVulnPrevalidation(ctx, id.TenantID, id.AccountID, current, snapshot)
	current.CompositionDigest, current.CompositionSnapshot = "", nil
	if current.PrevalidateStatus == VulnPrevalidatePassed {
		current.CompositionDigest, current.CompositionSnapshot = snapshot.Digest, &snapshot
	}
	if err := s.store.TenantTx(ctx, id.TenantID, func(ctx context.Context, tx TxStore) error {
		var err error
		current, err = tx.SetVulnProblemPrevalidate(ctx, current)
		if err != nil {
			return err
		}
		return s.writeAudit(ctx, id.TenantID, id.AccountID, contracts.RoleNumTeacher, "contest.vuln_problem.prevalidate", auditTargetVulnProblem, current.ID, map[string]any{"status": current.PrevalidateStatus})
	}); err != nil {
		return VulnProblemDTO{}, err
	}
	return vulnProblemDTOFromModel(current), nil
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
	if item.Status == VulnProblemStatusFinalized {
		return vulnProblemDTOFromModel(item), nil
	}
	if item.Status != VulnProblemStatusDraft {
		return VulnProblemDTO{}, apperr.ErrContestVulnPrevalidateFailed
	}
	if item.RuntimeMode == VulnRuntimeForked {
		if err := validateVulnForkSnapshot(item); err != nil {
			return VulnProblemDTO{}, err
		}
	}
	if item.CompositionSnapshot == nil || item.CompositionDigest == "" {
		return VulnProblemDTO{}, apperr.ErrContestVulnPrevalidateFailed
	}
	body, err := jsonx.CloneObjectStrict(item.DraftBody)
	if err != nil {
		return VulnProblemDTO{}, apperr.ErrContestVulnFinalizeFailed.WithCause(err)
	}
	// 内容声明不锁定使用场景；竞赛或实验发布时再按本场景编译访问边界。
	rawComposition, err := json.Marshal(item.CompositionSnapshot.Spec)
	if err != nil {
		return VulnProblemDTO{}, apperr.ErrContestVulnFinalizeFailed.WithCause(err)
	}
	composition, err := jsonx.ObjectMapStrict(rawComposition)
	if err != nil {
		return VulnProblemDTO{}, apperr.ErrContestVulnFinalizeFailed.WithCause(err)
	}
	delete(composition, "access_profile")
	body["composition"] = composition
	judgeConfig, ok := body["judge_config"].(map[string]any)
	if !ok {
		return VulnProblemDTO{}, apperr.ErrContestVulnPrevalidateFailed
	}
	expectation, ok := judgeConfig["expectation"].(map[string]any)
	if !ok {
		expectation = map[string]any{}
	}
	delete(expectation, "chain_steps")
	expectation["assertions"] = jsonx.SliceFromAny(body["assertions"])
	judgeConfig["expectation"] = expectation
	body["judge_config"] = judgeConfig
	for _, key := range []string{"positive_steps", "assertions"} {
		delete(body, key)
	}
	importCtx, err := auth.WithServiceIdentity(ctx, id.TenantID, fmt.Sprintf("contest:%04d:vuln-finalize:%s", timex.Now().Year(), ids.Format(item.ID)))
	if err != nil {
		return VulnProblemDTO{}, apperr.ErrContestVulnFinalizeFailed.WithCause(err)
	}
	contentSnapshot, err := s.contentImport.SystemImportContent(importCtx, contracts.ContentSystemImportRequest{TenantID: id.TenantID, Code: stableContestCode(item), Version: "1.0.0", Type: contentTypeContestProblem, Title: item.Title, Difficulty: contentDifficultyBasic, AuthorID: id.AccountID, AuthorType: contentAuthorExternal, Visibility: contentVisibilityTenant, Body: body, SensitiveFields: []string{"answer", "flag", "judge"}, AutoPublish: true, SystemImportNote: map[string]any{"source": "contest_vuln_problem", "vuln_problem_id": item.ID}})
	if err != nil {
		return VulnProblemDTO{}, apperr.ErrContestVulnFinalizeFailed.WithCause(err)
	}
	if err := s.store.TenantTx(ctx, id.TenantID, func(ctx context.Context, tx TxStore) error {
		var err error
		item, err = tx.FinalizeVulnProblem(ctx, id.TenantID, problemID, contentSnapshot.ItemCode, contentSnapshot.ItemVersion, item.UpdatedAt)
		if err != nil {
			return err
		}
		return s.writeAudit(ctx, id.TenantID, id.AccountID, contracts.RoleNumTeacher, "contest.vuln_problem.finalize", auditTargetVulnProblem, item.ID, map[string]any{"item_code": item.ContentItemCode, "item_version": item.ContentItemVersion})
	}); err != nil {
		return VulnProblemDTO{}, err
	}
	return vulnProblemDTOFromModel(item), nil
}

// fetchVulnCases 根据源配置拉取并解析漏洞案例。
func (s *Service) fetchVulnCases(ctx context.Context, source VulnSource) ([]VulnProblem, error) {
	cfg, err := parseVulnSourceConfig(source.Config, s.cfg.VulnSourceTimeoutSeconds)
	if err != nil {
		return nil, err
	}
	var body io.Reader
	if cfg.method == http.MethodPost {
		payload, exists := source.Config["body"]
		if !exists {
			payload = map[string]any{}
		}
		raw, err := json.Marshal(payload)
		if err != nil {
			return nil, apperr.ErrContestVulnSourceInvalid.WithCause(err)
		}
		body = bytes.NewReader(raw)
	}
	req, err := http.NewRequestWithContext(ctx, cfg.method, cfg.endpoint, body)
	if err != nil {
		return nil, apperr.ErrContestVulnSourceInvalid.WithCause(err)
	}
	req.Header.Set("Accept", "application/json")
	if cfg.method == http.MethodPost {
		req.Header.Set("Content-Type", "application/json")
	}
	for key, value := range cfg.headers {
		if strings.TrimSpace(value) != "" {
			req.Header.Set(key, value)
		}
	}
	client, err := netx.NewPublicHTTPClient(time.Duration(cfg.timeoutSeconds) * time.Second)
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
	raw, sizeResult, err := upload.ReadBounded(resp.Body, s.cfg.VulnSourceMaxResponseBytes)
	if err != nil {
		return nil, apperr.ErrContestVulnSourceReadFailed.WithCause(err)
	}
	if sizeResult == upload.SizeTooLarge {
		return nil, apperr.ErrContestVulnSourceTooLarge
	}
	if sizeResult == upload.SizeEmpty {
		return nil, apperr.ErrContestVulnSourceJSONInvalid
	}
	var payload any
	if err := jsonx.DecodeStrict(raw, &payload); err != nil {
		return nil, apperr.ErrContestVulnSourceJSONInvalid.WithCause(err)
	}
	nodes, err := selectCases(payload, cfg.casesPath)
	if err != nil {
		return nil, err
	}
	out := make([]VulnProblem, 0, len(nodes))
	for _, node := range nodes {
		item, err := vulnProblemFromExternal(node, cfg.mapping, source.DefaultLevel)
		if err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	return out, nil
}

// runVulnPrevalidation 在隔离沙箱中执行正向 PoC 与反向不误判验证。
func (s *Service) runVulnPrevalidation(ctx context.Context, tenantID, accountID int64, problem VulnProblem, snapshot contracts.SandboxCompositionSnapshot) (int16, map[string]any) {
	detail := map[string]any{"positive": map[string]any{}, "negative": map[string]any{}}
	if err := s.sandbox.PrepareCompositions(ctx, []string{snapshot.Digest}); err != nil {
		logging.ErrorContext(ctx, "vulnerability prevalidation composition preparation failed", apperr.AsAppError(err).LogString(), slog.Int64("tenant_id", tenantID), slog.Int64("problem_id", problem.ID))
		detail["error"] = safeDetailError(err)
		detail["trace_id"] = logging.TraceFromContext(ctx)
		return VulnPrevalidateFailed, detail
	}
	positive, err := s.runVulnValidationCase(ctx, tenantID, accountID, problem, snapshot, "positive", true)
	detail["positive"] = positive
	if err != nil {
		logging.ErrorContext(ctx, "vulnerability prevalidation failed", apperr.AsAppError(err).LogString(), slog.Int64("tenant_id", tenantID), slog.Int64("problem_id", problem.ID), slog.String("phase", "positive"))
		detail["error"] = safeDetailError(err)
		detail["trace_id"] = logging.TraceFromContext(ctx)
		return VulnPrevalidateFailed, detail
	}
	negative, err := s.runVulnValidationCase(ctx, tenantID, accountID, problem, snapshot, "negative", false)
	detail["negative"] = negative
	if err != nil {
		logging.ErrorContext(ctx, "vulnerability prevalidation failed", apperr.AsAppError(err).LogString(), slog.Int64("tenant_id", tenantID), slog.Int64("problem_id", problem.ID), slog.String("phase", "negative"))
		detail["error"] = safeDetailError(err)
		detail["trace_id"] = logging.TraceFromContext(ctx)
		return VulnPrevalidateFailed, detail
	}
	if !boolFromMap(positive, "passed") || !boolFromMap(negative, "passed") {
		return VulnPrevalidateFailed, detail
	}
	return VulnPrevalidatePassed, detail
}

// runVulnValidationCase 执行一条正向或反向预验证用例。
func (s *Service) runVulnValidationCase(ctx context.Context, tenantID, accountID int64, problem VulnProblem, snapshot contracts.SandboxCompositionSnapshot, phase string, positive bool) (result map[string]any, retErr error) {
	steps, err := validationSteps(problem.DraftBody, "positive_steps")
	if err != nil {
		return nil, err
	}
	assertions, err := validationSteps(problem.DraftBody, "assertions")
	if err != nil || len(assertions) == 0 {
		return nil, apperr.ErrContestVulnProblemInvalid
	}
	sourceRef := fmt.Sprintf("contest:%04d:vuln-prevalidate-%s:%s", timex.Now().Year(), phase, ids.Format(problem.ID))
	scopeRef := sourceRef + ":scope:" + ids.Format(s.ids.Generate())
	// 创建响应丢失时也按本次唯一作用域回收,不依赖调用方拿到沙箱编号。
	defer func() {
		cleanupCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), time.Duration(s.cfg.BattleSandboxReadyTimeoutSeconds)*time.Second)
		defer cancel()
		if recycleErr := s.sandbox.RecycleByScopeRef(cleanupCtx, contracts.SandboxRecycleRequest{TenantID: tenantID, ScopeRef: scopeRef, SourceRef: sourceRef, Reason: "vuln_prevalidate"}); recycleErr != nil {
			if retErr != nil {
				retErr = apperr.ErrContestSandboxUnavailable.WithCause(fmt.Errorf("漏洞预验证失败: %w; 回收沙箱失败: %w", retErr, recycleErr))
				return
			}
			retErr = apperr.ErrContestSandboxUnavailable.WithCause(recycleErr)
		}
	}()
	info, err := s.sandbox.CreateSandbox(ctx, contracts.SandboxCreateRequest{TenantID: tenantID, CompositionSnapshot: snapshot, InitCodeRef: snapshot.Spec.InitCodeRef, InitScriptRef: snapshot.Spec.InitScriptRef, OwnerAccountID: accountID, SourceRef: sourceRef, ScopeRef: scopeRef, KeepAlive: false, SnapshotEnabled: false})
	if err != nil {
		return nil, apperr.ErrContestSandboxUnavailable.WithCause(err)
	}
	if err := s.waitSandboxReady(ctx, tenantID, info.SandboxID); err != nil {
		return nil, err
	}
	runtimeInstance := strings.TrimSpace(snapshot.Spec.WorkspaceRuntimeInstance)
	if runtimeInstance == "" {
		return nil, apperr.ErrContestVulnProblemInvalid
	}
	if positive {
		for _, step := range steps {
			if err := s.runVulnChainStep(ctx, tenantID, info.SandboxID, sourceRef, runtimeInstance, step); err != nil {
				return nil, err
			}
		}
	}
	results, err := s.checkVulnAssertions(ctx, tenantID, info.SandboxID, sourceRef, runtimeInstance, assertions, positive)
	if err != nil {
		return nil, err
	}
	return map[string]any{"passed": allAssertionResults(results), "assertions": results}, nil
}

// runVulnChainStep 调用 M2 链能力执行预验证步骤。
func (s *Service) runVulnChainStep(ctx context.Context, tenantID, sandboxID int64, sourceRef, runtimeInstance string, step map[string]any) error {
	runtimeInstance, err := contracts.ChainRuntimeInstance(step, runtimeInstance)
	if err != nil {
		return apperr.ErrContestVulnProblemInvalid.WithCause(err)
	}
	switch strings.TrimSpace(jsonx.StringFromAny(step["op"])) {
	case VulnChainOperationDeploy:
		_, err := s.sandbox.ChainDeploy(ctx, contracts.SandboxChainDeployRequest{TenantID: tenantID, SandboxID: sandboxID, SourceRef: sourceRef, RuntimeInstance: runtimeInstance, Payload: jsonx.ObjectFromAny(step["payload"])})
		return err
	case VulnChainOperationTx:
		_, err := s.sandbox.ChainSendTx(ctx, contracts.SandboxChainTxRequest{TenantID: tenantID, SandboxID: sandboxID, SourceRef: sourceRef, RuntimeInstance: runtimeInstance, Payload: jsonx.ObjectFromAny(step["payload"])})
		return err
	case VulnChainOperationReset:
		return s.sandbox.ChainReset(ctx, contracts.SandboxChainResetRequest{TenantID: tenantID, SandboxID: sandboxID, SourceRef: sourceRef, RuntimeInstance: runtimeInstance})
	case VulnChainOperationQuery:
		target := strings.TrimSpace(jsonx.StringField(step, "target"))
		if target == "" {
			return apperr.ErrContestVulnProblemInvalid
		}
		_, err := s.sandbox.ChainQuery(ctx, contracts.SandboxChainQueryRequest{TenantID: tenantID, SandboxID: sandboxID, SourceRef: sourceRef, RuntimeInstance: runtimeInstance, Target: target})
		return err
	default:
		return apperr.ErrContestVulnProblemInvalid
	}
}

// checkVulnAssertions 检查正向应通过、反向应全部不通过的断言集合。
func (s *Service) checkVulnAssertions(ctx context.Context, tenantID, sandboxID int64, sourceRef, runtimeInstance string, assertions []map[string]any, positive bool) ([]map[string]any, error) {
	if len(assertions) == 0 {
		return nil, apperr.ErrContestVulnProblemInvalid
	}
	out := make([]map[string]any, 0, len(assertions))
	for _, raw := range assertions {
		assertion, err := chainassert.FromMap(raw)
		if err != nil {
			return nil, apperr.ErrContestVulnProblemInvalid.WithCause(err)
		}
		instance, err := contracts.ChainRuntimeInstance(raw, runtimeInstance)
		if err != nil {
			return nil, apperr.ErrContestVulnProblemInvalid.WithCause(err)
		}
		actual, err := s.sandbox.ChainQuery(ctx, contracts.SandboxChainQueryRequest{TenantID: tenantID, SandboxID: sandboxID, SourceRef: sourceRef, RuntimeInstance: instance, Target: assertion.Target})
		if err != nil {
			return nil, apperr.ErrContestSandboxUnavailable.WithCause(err)
		}
		result, err := chainassert.Check(assertion, actual)
		if err != nil {
			return nil, apperr.ErrContestSandboxUnavailable.WithCause(err)
		}
		passed := result.Passed
		if !positive {
			passed = !result.Passed
		}
		out = append(out, map[string]any{"case": result.Case, "passed": passed, "expected_label": result.ExpectedLabel, "actual": result.Actual, "hint": result.Hint})
	}
	return out, nil
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

// safeDetailError 只返回应用错误的用户向文案,内部错误链交由执行日志记录。
func safeDetailError(err error) string {
	if err == nil {
		return ""
	}
	return apperr.AsAppError(err).UserMessage()
}
