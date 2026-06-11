// sim row_convert 文件负责 sqlc 行到 M4 领域模型的纯转换。
package sim

import (
	"encoding/json"
	"time"

	"chaimir/internal/modules/sim/internal/sqlcgen"

	"github.com/jackc/pgx/v5/pgtype"
)

// packageFromRow 转换平台级仿真包行。
func packageFromRow(row sqlcgen.SimPackage) (Package, error) {
	scale, err := decodeMap(row.ScaleLimit)
	if err != nil {
		return Package{}, err
	}
	backendConfig, err := decodeMap(row.BackendConfig)
	if err != nil {
		return Package{}, err
	}
	return Package{
		ID:             row.ID,
		Code:           row.Code,
		Version:        row.Version,
		Name:           row.Name,
		Category:       row.Category,
		Compute:        row.Compute,
		ScaleLimit:     scale,
		BundleKey:      row.BundleKey,
		BundleHash:     row.BundleHash,
		BackendAdapter: textValue(row.BackendAdapter),
		BackendConfig:  backendConfig,
		AuthorType:     row.AuthorType,
		AuthorID:       int64Value(row.AuthorID),
		Status:         row.Status,
		CreatedAt:      timeFromPg(row.CreatedAt),
		UpdatedAt:      timeFromPg(row.UpdatedAt),
	}, nil
}

// reviewFromRow 转换审核记录并解析审核报告。
func reviewFromRow(row sqlcgen.SimPackageReview) (Review, error) {
	report, err := reportFromJSON(row.PreviewReport)
	if err != nil {
		return Review{}, err
	}
	return Review{
		ID:            row.ID,
		PackageID:     row.PackageID,
		SubmitterID:   row.SubmitterID,
		PreviewReport: report,
		ReviewerID:    int64Value(row.ReviewerID),
		Result:        row.Result,
		Comment:       textValue(row.Comment),
		CreatedAt:     timeFromPg(row.CreatedAt),
		UpdatedAt:     timeFromPg(row.UpdatedAt),
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
	params, err := decodeMap(row.InitParams)
	if err != nil {
		return Session{}, err
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
		CreatedAt:      timeFromPg(row.CreatedAt),
		UpdatedAt:      timeFromPg(row.UpdatedAt),
	}, nil
}

// sessionWithPackageFromRow 转换回放所需的会话和包摘要。
func sessionWithPackageFromRow(row sqlcgen.GetSimSessionWithPackageRow) (SessionWithPackage, error) {
	session, err := sessionFromRow(sqlcgen.SimSession{ID: row.ID, TenantID: row.TenantID, PackageID: row.PackageID, SourceRef: row.SourceRef, OwnerAccountID: row.OwnerAccountID, Seed: row.Seed, InitParams: row.InitParams, Compute: row.Compute, Status: row.Status, CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt})
	if err != nil {
		return SessionWithPackage{}, err
	}
	backendConfig, err := decodeMap(row.BackendConfig)
	if err != nil {
		return SessionWithPackage{}, err
	}
	return SessionWithPackage{Session: session, PackageCode: row.Code, PackageVersion: row.Version, PackageName: row.Name, Category: row.Category, BundleKey: row.BundleKey, BundleHash: row.BundleHash, BackendAdapter: textValue(row.BackendAdapter), BackendConfig: backendConfig, PackageStatus: row.PackageStatus}, nil
}

// actionFromRow 转换操作序列行。
func actionFromRow(row sqlcgen.SimActionLog) (Action, error) {
	payload, err := decodeMap(row.Payload)
	if err != nil {
		return Action{}, err
	}
	return Action{ID: row.ID, TenantID: row.TenantID, SessionID: row.SessionID, Seq: row.Seq, AtTick: row.AtTick, EventType: row.EventType, Payload: payload, CreatedAt: timeFromPg(row.CreatedAt)}, nil
}

// shareFromRow 转换分享码索引。
func shareFromRow(row sqlcgen.SimShare) Share {
	return Share{ID: row.ID, TenantID: row.TenantID, SessionID: row.SessionID, Code: row.Code, CreatedBy: row.CreatedBy, Status: row.Status, ExpireAt: timeFromPg(row.ExpireAt), CreatedAt: timeFromPg(row.CreatedAt), UpdatedAt: timeFromPg(row.UpdatedAt)}
}

// decodeMap 解码 JSONB 对象为空 map。
func decodeMap(raw []byte) (map[string]any, error) {
	if len(raw) == 0 {
		return map[string]any{}, nil
	}
	out := map[string]any{}
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// reportFromJSON 解码审核报告。
func reportFromJSON(raw []byte) (ValidationReport, error) {
	var out ValidationReport
	if len(raw) == 0 {
		return out, nil
	}
	if err := json.Unmarshal(raw, &out); err != nil {
		return ValidationReport{}, err
	}
	return out, nil
}

// textParam 构造可空 text 参数。
func textParam(value string) pgtype.Text {
	return pgtype.Text{String: value, Valid: value != ""}
}

// int64Param 构造可空 int8 参数。
func int64Param(value int64) pgtype.Int8 {
	return pgtype.Int8{Int64: value, Valid: value > 0}
}

// timeParam 构造可空 timestamptz 参数。
func timeParam(value time.Time) pgtype.Timestamptz {
	return pgtype.Timestamptz{Time: value.UTC(), Valid: !value.IsZero()}
}

// textValue 读取可空 text。
func textValue(value pgtype.Text) string {
	if value.Valid {
		return value.String
	}
	return ""
}

// int64Value 读取可空 int8。
func int64Value(value pgtype.Int8) int64 {
	if value.Valid {
		return value.Int64
	}
	return 0
}

// timeFromPg 读取 pgtype 时间。
func timeFromPg(value pgtype.Timestamptz) time.Time {
	if value.Valid {
		return value.Time.UTC()
	}
	return time.Time{}
}
