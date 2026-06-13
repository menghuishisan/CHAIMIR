// transfer store 文件负责统一导入导出任务的持久化访问边界。
package transfer

import (
	"context"
	"fmt"
	"time"

	"chaimir/internal/platform/db"
	"chaimir/internal/platform/timex"
	"chaimir/internal/platform/transfer/internal/sqlcgen"
	"chaimir/pkg/apperr"

	"github.com/jackc/pgx/v5"
)

// Store 描述统一导入导出中心可被服务层使用的持久化契约。
type Store interface {
	CreateTask(ctx context.Context, task Task) (Task, error)
	GetTask(ctx context.Context, tenantID, taskID int64) (Task, error)
	ListTasks(ctx context.Context, query ListTasksQuery) ([]Task, error)
	UpdateTask(ctx context.Context, task Task) (Task, error)
	ClaimDueTasks(ctx context.Context, tenantID int64, nowAt time.Time, limit int32) ([]Task, error)
}

// ListTasksQuery 描述当前账号查询统一导入导出任务的分页与过滤条件。
type ListTasksQuery struct {
	TenantID  int64
	AccountID int64
	Channel   Channel
	Status    Status
	Limit     int32
	Offset    int32
}

type store struct {
	db *db.DB
}

// NewStore 构造统一导入导出任务持久化入口。
func NewStore(database *db.DB) Store {
	return &store{db: database}
}

// CreateTask 在租户 RLS 边界内创建导入导出任务。
func (s *store) CreateTask(ctx context.Context, task Task) (Task, error) {
	if s.db == nil {
		return Task{}, fmt.Errorf("transfer store 缺少 database")
	}
	var out Task
	err := s.withTaskTx(ctx, task.TenantID, func(ctx context.Context, tx pgx.Tx) error {
		row, err := sqlcgen.New(tx).CreateTransferTask(ctx, sqlcgen.CreateTransferTaskParams{
			ID:                  task.TaskID,
			TenantID:            task.TenantID,
			AccountID:           task.AccountID,
			Channel:             string(task.Channel),
			Subject:             task.Subject,
			Status:              string(task.Status),
			ContentType:         task.ContentType,
			FileName:            task.FileName,
			AttemptCount:        int32(task.AttemptCount),
			MaxAttempts:         int32(task.MaxAttempts),
			LastError:           task.LastError,
			ArtifactRef:         task.Artifact.ObjectRef,
			ArtifactSize:        task.Artifact.Size,
			ArtifactContentType: task.Artifact.ContentType,
			ArtifactFileName:    task.Artifact.FileName,
			CreatedAt:           timex.RequiredTimestamptz(task.CreatedAt),
			UpdatedAt:           timex.RequiredTimestamptz(task.UpdatedAt),
			CompletedAt:         timex.Timestamptz(task.CompletedAt),
			NextAttemptAfter:    timex.Timestamptz(task.NextAttemptAfter),
		})
		if err != nil {
			return err
		}
		out = taskFromRow(row)
		return nil
	})
	if err != nil {
		return Task{}, mapStoreError(err)
	}
	return out, nil
}

// GetTask 读取租户内单个导入导出任务。
func (s *store) GetTask(ctx context.Context, tenantID, taskID int64) (Task, error) {
	if s.db == nil {
		return Task{}, fmt.Errorf("transfer store 缺少 database")
	}
	var out Task
	err := s.withTaskTx(ctx, tenantID, func(ctx context.Context, tx pgx.Tx) error {
		row, err := sqlcgen.New(tx).GetTransferTask(ctx, sqlcgen.GetTransferTaskParams{TenantID: tenantID, ID: taskID})
		if err != nil {
			return err
		}
		out = taskFromRow(row)
		return nil
	})
	if err != nil {
		return Task{}, mapStoreError(err)
	}
	return out, nil
}

// ListTasks 查询当前账号名下的导入导出任务。
func (s *store) ListTasks(ctx context.Context, query ListTasksQuery) ([]Task, error) {
	if s.db == nil {
		return nil, fmt.Errorf("transfer store 缺少 database")
	}
	var out []Task
	err := s.withTaskTx(ctx, query.TenantID, func(ctx context.Context, tx pgx.Tx) error {
		rows, err := sqlcgen.New(tx).ListTransferTasks(ctx, sqlcgen.ListTransferTasksParams{
			TenantID:  query.TenantID,
			AccountID: query.AccountID,
			Column3:   string(query.Channel),
			Column4:   string(query.Status),
			Limit:     query.Limit,
			Offset:    query.Offset,
		})
		if err != nil {
			return err
		}
		out = tasksFromRows(rows)
		return nil
	})
	if err != nil {
		return nil, mapStoreError(err)
	}
	return out, nil
}

// UpdateTask 更新导入导出任务状态、重试信息和产物引用。
func (s *store) UpdateTask(ctx context.Context, task Task) (Task, error) {
	if s.db == nil {
		return Task{}, fmt.Errorf("transfer store 缺少 database")
	}
	var out Task
	err := s.withTaskTx(ctx, task.TenantID, func(ctx context.Context, tx pgx.Tx) error {
		row, err := sqlcgen.New(tx).UpdateTransferTask(ctx, sqlcgen.UpdateTransferTaskParams{
			TenantID:            task.TenantID,
			ID:                  task.TaskID,
			Status:              string(task.Status),
			AttemptCount:        int32(task.AttemptCount),
			MaxAttempts:         int32(task.MaxAttempts),
			LastError:           task.LastError,
			ArtifactRef:         task.Artifact.ObjectRef,
			ArtifactSize:        task.Artifact.Size,
			ArtifactContentType: task.Artifact.ContentType,
			ArtifactFileName:    task.Artifact.FileName,
			UpdatedAt:           timex.RequiredTimestamptz(task.UpdatedAt),
			CompletedAt:         timex.Timestamptz(task.CompletedAt),
			NextAttemptAfter:    timex.Timestamptz(task.NextAttemptAfter),
		})
		if err != nil {
			return err
		}
		out = taskFromRow(row)
		return nil
	})
	if err != nil {
		return Task{}, mapStoreError(err)
	}
	return out, nil
}

// ClaimDueTasks 供后台执行器批量领取到期任务。
func (s *store) ClaimDueTasks(ctx context.Context, tenantID int64, nowAt time.Time, limit int32) ([]Task, error) {
	if s.db == nil {
		return nil, fmt.Errorf("transfer store 缺少 database")
	}
	if tenantID < 0 || nowAt.IsZero() {
		return nil, apperr.ErrTransferTaskInvalid
	}
	if limit <= 0 {
		return nil, apperr.ErrTransferTaskInvalid
	}
	var out []Task
	err := s.withTaskTx(ctx, tenantID, func(ctx context.Context, tx pgx.Tx) error {
		rows, err := sqlcgen.New(tx).ClaimDueTransferTasks(ctx, sqlcgen.ClaimDueTransferTasksParams{ClaimTenantID: tenantID, NowAt: timex.Timestamptz(nowAt), BatchLimit: limit})
		if err != nil {
			return err
		}
		out = tasksFromRows(rows)
		return nil
	})
	if err != nil {
		return nil, mapStoreError(err)
	}
	return out, nil
}

// withTaskTx 按任务租户边界选择平台事务或租户 RLS 事务。
func (s *store) withTaskTx(ctx context.Context, tenantID int64, fn func(context.Context, pgx.Tx) error) error {
	if tenantID < 0 {
		return apperr.ErrTransferTaskInvalid
	}
	if tenantID == 0 {
		return s.db.WithAppTx(ctx, fn)
	}
	return s.db.WithTenantTxID(ctx, tenantID, fn)
}

// mapStoreError 将数据库未命中归一为 transfer 领域错误码。
func mapStoreError(err error) error {
	if err == nil {
		return nil
	}
	if db.IsNoRows(err) {
		return apperr.ErrTransferTaskNotFound
	}
	return apperr.ErrTransferTaskInvalid.WithCause(err)
}
