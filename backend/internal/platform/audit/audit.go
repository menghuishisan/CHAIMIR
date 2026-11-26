// Package audit 构造审计条目，由身份模块统一写入。
package audit

import (
	"context"
	"fmt"
	"strings"

	"chaimir/internal/platform/jsonx"
	"chaimir/internal/platform/response"
	"chaimir/pkg/apperr"
)

// Entry 表示一条要写入 M1 audit_log 的审计记录。
type Entry struct {
	TenantID   int64
	ActorID    int64
	ActorRole  int16
	Action     string
	TargetType string
	TargetID   int64
	Detail     string
	IP         string
	TraceID    string
}

// Writer 是审计写入能力契约,由 identity 模块实现并在装配时注入。
type Writer interface {
	// Write 写入一条审计记录到全平台唯一 audit_log。
	Write(ctx context.Context, e Entry) error
}

type requestIPKey struct{}

// WithRequestIP 保存 HTTP 入口解析的客户端 IP。
func WithRequestIP(ctx context.Context, ip string) context.Context {
	return context.WithValue(ctx, requestIPKey{}, ip)
}

// BuildEntry 校验动作与目标，从当前上下文补齐审计 IP 和追踪编号。
func BuildEntry(ctx context.Context, tenantID, actorID int64, actorRole int16, action, targetType string, targetID int64, detail map[string]any) (Entry, error) {
	action = strings.TrimSpace(action)
	targetType = strings.TrimSpace(targetType)
	if action == "" || targetType == "" {
		return Entry{}, fmt.Errorf("审计条目缺少 action 或 target_type")
	}
	detailJSON, err := jsonx.ObjectBytes(detail, apperr.ErrInternal)
	if err != nil {
		return Entry{}, err
	}
	ip, _ := ctx.Value(requestIPKey{}).(string)
	return Entry{
		TenantID:   tenantID,
		ActorID:    actorID,
		ActorRole:  actorRole,
		Action:     action,
		TargetType: targetType,
		TargetID:   targetID,
		Detail:     string(detailJSON),
		IP:         ip,
		TraceID:    response.TraceFromContext(ctx),
	}, nil
}
