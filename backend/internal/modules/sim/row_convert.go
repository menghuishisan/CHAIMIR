// sim row_convert 文件负责 sqlc 行到 M4 领域模型的纯转换。
package sim

import (
	"fmt"

	"chaimir/internal/modules/sim/internal/sqlcgen"
	"chaimir/internal/platform/jsonx"
	"chaimir/internal/platform/pgtypex"
	"chaimir/internal/platform/timex"
	"chaimir/pkg/apperr"
)

// packageFromRow 转换平台级仿真包行。
func packageFromRow(row sqlcgen.SimPackage) (Package, error) {
	scale, err := jsonx.ObjectMapStrict(row.ScaleLimit)
	if err != nil {
		return Package{}, apperr.ErrSimPackageDataCorrupt.WithCause(fmt.Errorf("仿真包 %d 的 scale_limit 数据异常: %w", row.ID, err))
	}
	backendConfig, err := jsonx.ObjectMapStrict(row.BackendConfig)
	if err != nil {
		return Package{}, apperr.ErrSimPackageDataCorrupt.WithCause(fmt.Errorf("仿真包 %d 的 backend_config 数据异常: %w", row.ID, err))
	}
	interactionSchema, err := decodeInteractionSchema(row.InteractionSchema)
	if err != nil {
		return Package{}, apperr.ErrSimPackageDataCorrupt.WithCause(fmt.Errorf("仿真包 %d 的 interaction_schema 数据异常: %w", row.ID, err))
	}
	codeTrace, err := decodeCodeTraceAudit(row.CodeTrace)
	if err != nil {
		return Package{}, apperr.ErrSimPackageDataCorrupt.WithCause(fmt.Errorf("仿真包 %d 的 code_trace 数据异常: %w", row.ID, err))
	}
	return Package{
		ID:                row.ID,
		Code:              row.Code,
		Version:           row.Version,
		Name:              row.Name,
		Category:          row.Category,
		Compute:           row.Compute,
		ScaleLimit:        scale,
		BundleKey:         row.BundleKey,
		BundleHash:        row.BundleHash,
		BackendAdapter:    pgtypex.TextValue(row.BackendAdapter),
		BackendConfig:     backendConfig,
		InteractionSchema: interactionSchema,
		CodeTrace:         codeTrace,
		AuthorType:        row.AuthorType,
		AuthorID:          pgtypex.Int8Value(row.AuthorID),
		Status:            row.Status,
		CreatedAt:         timex.FromTimestamptz(row.CreatedAt),
		UpdatedAt:         timex.FromTimestamptz(row.UpdatedAt),
	}, nil
}

// reviewFromRow 转换审核记录并解析审核报告。
func reviewFromRow(row sqlcgen.SimPackageReview) (Review, error) {
	report, err := reportFromJSON(row.PreviewReport)
	if err != nil {
		return Review{}, apperr.ErrSimReviewDataCorrupt.WithCause(fmt.Errorf("审核记录 %d 的预览报告数据异常: %w", row.ID, err))
	}
	return Review{
		ID:            row.ID,
		PackageID:     row.PackageID,
		SubmitterID:   row.SubmitterID,
		PreviewReport: report,
		ReviewerID:    pgtypex.Int8Value(row.ReviewerID),
		Result:        row.Result,
		Comment:       pgtypex.TextValue(row.Comment),
		CreatedAt:     timex.FromTimestamptz(row.CreatedAt),
		UpdatedAt:     timex.FromTimestamptz(row.UpdatedAt),
	}, nil
}

// reviewInfoFromRow 转换带包摘要的审核列表行。
func reviewInfoFromRow(row sqlcgen.ListSimReviewsRow) (ReviewInfo, error) {
	review, err := reviewFromRow(sqlcgen.SimPackageReview{
		ID:            row.ID,
		PackageID:     row.PackageID,
		SubmitterID:   row.SubmitterID,
		PreviewReport: row.PreviewReport,
		ReviewerID:    row.ReviewerID,
		Result:        row.Result,
		Comment:       row.Comment,
		CreatedAt:     row.CreatedAt,
		UpdatedAt:     row.UpdatedAt,
	})
	if err != nil {
		return ReviewInfo{}, err
	}
	return ReviewInfo{Review: review, PackageCode: row.Code, PackageVersion: row.Version, PackageName: row.Name, Category: row.Category, Compute: row.Compute, PackageStatus: row.Status}, nil
}

// sessionFromRow 转换仿真会话行。
func sessionFromRow(row sqlcgen.SimSession) (Session, error) {
	params, err := jsonx.ObjectMapStrict(row.InitParams)
	if err != nil {
		return Session{}, apperr.ErrSimSessionDataCorrupt.WithCause(fmt.Errorf("仿真会话 %d 的 init_params 数据异常: %w", row.ID, err))
	}
	return Session{
		ID:             row.ID,
		TenantID:       row.TenantID,
		PackageID:      row.PackageID,
		SourceRef:      row.SourceRef,
		OwnerAccountID: row.OwnerAccountID,
		Seed:           row.Seed,
		InitParams:     params,
		Compute:        row.Compute,
		Status:         row.Status,
		CreatedAt:      timex.FromTimestamptz(row.CreatedAt),
		UpdatedAt:      timex.FromTimestamptz(row.UpdatedAt),
	}, nil
}

// sessionWithPackageFromRow 转换回放所需的会话和包摘要。
func sessionWithPackageFromRow(row sqlcgen.GetSimSessionWithPackageRow) (SessionWithPackage, error) {
	session, err := sessionFromRow(sqlcgen.SimSession{ID: row.ID, TenantID: row.TenantID, PackageID: row.PackageID, SourceRef: row.SourceRef, OwnerAccountID: row.OwnerAccountID, Seed: row.Seed, InitParams: row.InitParams, Compute: row.Compute, Status: row.Status, CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt})
	if err != nil {
		return SessionWithPackage{}, err
	}
	scaleLimit, err := jsonx.ObjectMapStrict(row.ScaleLimit)
	if err != nil {
		return SessionWithPackage{}, apperr.ErrSimPackageDataCorrupt.WithCause(fmt.Errorf("仿真包 %d 的 scale_limit 数据异常: %w", row.PackageID, err))
	}
	backendConfig, err := jsonx.ObjectMapStrict(row.BackendConfig)
	if err != nil {
		return SessionWithPackage{}, apperr.ErrSimPackageDataCorrupt.WithCause(fmt.Errorf("仿真包 %d 的 backend_config 数据异常: %w", row.PackageID, err))
	}
	interactionSchema, err := decodeInteractionSchema(row.InteractionSchema)
	if err != nil {
		return SessionWithPackage{}, apperr.ErrSimPackageDataCorrupt.WithCause(fmt.Errorf("仿真包 %d 的 interaction_schema 数据异常: %w", row.PackageID, err))
	}
	return SessionWithPackage{Session: session, PackageCode: row.Code, PackageVersion: row.Version, PackageName: row.Name, Category: row.Category, ScaleLimit: scaleLimit, BundleKey: row.BundleKey, BundleHash: row.BundleHash, BackendAdapter: pgtypex.TextValue(row.BackendAdapter), BackendConfig: backendConfig, InteractionSchema: interactionSchema, PackageStatus: row.PackageStatus}, nil
}

// actionFromRow 转换操作序列行。
func actionFromRow(row sqlcgen.SimActionLog) (Action, error) {
	payload, err := jsonx.ObjectMapStrict(row.Payload)
	if err != nil {
		return Action{}, apperr.ErrSimSessionDataCorrupt.WithCause(fmt.Errorf("仿真会话 %d 的操作记录 %d payload 数据异常: %w", row.SessionID, row.Seq, err))
	}
	return Action{ID: row.ID, TenantID: row.TenantID, SessionID: row.SessionID, Seq: row.Seq, AtTick: row.AtTick, EventType: row.EventType, Payload: payload, CreatedAt: timex.FromTimestamptz(row.CreatedAt)}, nil
}

// shareFromRow 转换分享码索引。
func shareFromRow(row sqlcgen.SimShare) Share {
	return Share{ID: row.ID, TenantID: row.TenantID, SessionID: row.SessionID, Code: row.Code, CreatedBy: row.CreatedBy, Status: row.Status, ExpireAt: timex.FromTimestamptz(row.ExpireAt), CreatedAt: timex.FromTimestamptz(row.CreatedAt), UpdatedAt: timex.FromTimestamptz(row.UpdatedAt)}
}

// reportFromJSON 解码审核报告。
func reportFromJSON(raw []byte) (ValidationReport, error) {
	var out ValidationReport
	if len(raw) == 0 {
		return out, nil
	}
	if err := jsonx.DecodeStrict(raw, &out); err != nil {
		return ValidationReport{}, err
	}
	return out, nil
}

// decodeInteractionSchema 解码交互白名单,并恢复参数索引。
func decodeInteractionSchema(raw []byte) (InteractionSchema, error) {
	if len(raw) == 0 {
		return normalizeInteractionSchema(InteractionSchema{}), nil
	}
	var out InteractionSchema
	if err := jsonx.DecodeStrict(raw, &out); err != nil {
		return InteractionSchema{}, err
	}
	return normalizeInteractionSchema(out), nil
}

// decodeCodeTraceAudit 解码代码追踪审核摘要。
func decodeCodeTraceAudit(raw []byte) (CodeTraceAudit, error) {
	if len(raw) == 0 {
		return CodeTraceAudit{}, nil
	}
	var out CodeTraceAudit
	if err := jsonx.DecodeStrict(raw, &out); err != nil {
		return CodeTraceAudit{}, err
	}
	return out, nil
}
