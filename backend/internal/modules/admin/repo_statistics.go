// 读取及保存 M9 周期统计快照。
package admin

import (
	"context"

	"chaimir/internal/modules/admin/internal/sqlcgen"
	"chaimir/internal/platform/ids"
	"chaimir/internal/platform/jsonx"
	"chaimir/internal/platform/pgtypex"
	"chaimir/internal/platform/timex"
	"chaimir/pkg/apperr"
)

// ListPlatformStatistics 查询运营统计时间序列。
func (t *txStore) ListPlatformStatistics(ctx context.Context, scope int16, tenantID int64, fromDate, toDate string) ([]StatisticsDTO, error) {
	from, err := timex.ParseDate(fromDate)
	if err != nil {
		return nil, apperr.ErrAdminStatisticsInvalid.WithCause(err)
	}
	to, err := timex.ParseDate(toDate)
	if err != nil {
		return nil, apperr.ErrAdminStatisticsInvalid.WithCause(err)
	}
	rows, err := t.q.ListPlatformStatistics(ctx, sqlcgen.ListPlatformStatisticsParams{Scope: scope, TenantID: pgtypex.Int8When(tenantID, tenantID > 0), FromDate: pgtypex.Date(from), ToDate: pgtypex.Date(to)})
	if err != nil {
		return nil, err
	}
	out := make([]StatisticsDTO, 0, len(rows))
	for _, row := range rows {
		metrics, err := decodeStoredObject(row.Metrics, apperr.ErrAdminStatisticsInvalid)
		if err != nil {
			return nil, err
		}
		out = append(out, StatisticsDTO{Scope: row.Scope, TenantID: ids.ID(pgtypex.Int8Value(row.TenantID)), Date: timex.DateOrEmpty(pgtypex.DateValue(row.StatDate)), Metrics: metrics})
	}
	return out, nil
}

// UpsertPlatformStatistics 写入或更新运营统计快照。
func (t *txStore) UpsertPlatformStatistics(ctx context.Context, id int64, scope int16, tenantID int64, statDate string, metrics map[string]any) (StatisticsDTO, error) {
	date, err := timex.ParseDate(statDate)
	if err != nil {
		return StatisticsDTO{}, apperr.ErrAdminStatisticsInvalid.WithCause(err)
	}
	data, err := jsonx.ObjectBytes(metrics, apperr.ErrAdminStatisticsInvalid)
	if err != nil {
		return StatisticsDTO{}, err
	}
	var row sqlcgen.PlatformStatistic
	if scope == ScopeGlobal {
		row, err = t.q.UpsertGlobalPlatformStatistics(ctx, sqlcgen.UpsertGlobalPlatformStatisticsParams{ID: id, StatDate: pgtypex.Date(date), Metrics: data})
	} else {
		row, err = t.q.UpsertTenantPlatformStatistics(ctx, sqlcgen.UpsertTenantPlatformStatisticsParams{ID: id, Scope: scope, TenantID: pgtypex.Int8When(tenantID, tenantID > 0), StatDate: pgtypex.Date(date), Metrics: data})
	}
	if err != nil {
		return StatisticsDTO{}, err
	}
	decodedMetrics, err := decodeStoredObject(row.Metrics, apperr.ErrAdminStatisticsInvalid)
	if err != nil {
		return StatisticsDTO{}, err
	}
	return StatisticsDTO{Scope: row.Scope, TenantID: ids.ID(pgtypex.Int8Value(row.TenantID)), Date: timex.DateOrEmpty(pgtypex.DateValue(row.StatDate)), Metrics: decodedMetrics}, nil
}
