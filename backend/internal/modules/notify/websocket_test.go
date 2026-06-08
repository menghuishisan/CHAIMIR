// M10 WebSocket 测试:覆盖 topic 订阅授权与租户隔离规则。
package notify

import (
	"testing"

	"chaimir/internal/platform/tenant"
	"chaimir/pkg/apperr"
)

// TestAuthorizeTopicRejectsCrossTenantInternalPrefix 确认客户端不能伪造内部租户 topic 前缀。
func TestAuthorizeTopicRejectsCrossTenantInternalPrefix(t *testing.T) {
	_, err := authorizeTopic(tenant.Identity{TenantID: 10, AccountID: 501}, "tenant:20:contest:55:leaderboard")
	if err == nil {
		t.Fatalf("expected tenant-prefixed topic to be rejected")
	}
	if ae, ok := apperr.As(err); !ok || ae.Code != apperr.ErrNotifyTopicForbidden.Code {
		t.Fatalf("expected topic forbidden error, got %v", err)
	}
}

// TestAuthorizeTopicAllowsOnlyOwnNotifyTopic 确认个人红点 topic 只能订阅本人。
func TestAuthorizeTopicAllowsOnlyOwnNotifyTopic(t *testing.T) {
	_, err := authorizeTopic(tenant.Identity{TenantID: 10, AccountID: 501}, "notify:502")
	if err == nil {
		t.Fatalf("expected other account notify topic to be rejected")
	}
	topic, err := authorizeTopic(tenant.Identity{TenantID: 10, AccountID: 501}, "notify:501")
	if err != nil {
		t.Fatalf("own notify topic rejected: %v", err)
	}
	if topic != "tenant:10:notify:501" {
		t.Fatalf("unexpected tenant topic: %s", topic)
	}
}

// TestAuthorizeTopicAllowsTenantScopedBusinessTopic 确认普通业务 topic 由 M10 加租户隔离前缀。
func TestAuthorizeTopicAllowsTenantScopedBusinessTopic(t *testing.T) {
	topic, err := authorizeTopic(tenant.Identity{TenantID: 10, AccountID: 501}, "contest:55:leaderboard")
	if err != nil {
		t.Fatalf("business topic rejected: %v", err)
	}
	if topic != "tenant:10:contest:55:leaderboard" {
		t.Fatalf("unexpected tenant topic: %s", topic)
	}
}

// TestSubscriptionProtocolErrorAckUsesNotifyCode 确认订阅协议错误使用 M10 专属错误码。
func TestSubscriptionProtocolErrorAckUsesNotifyCode(t *testing.T) {
	ack := subscriptionProtocolErrorAck("publish")
	if ack.Code != apperr.ErrNotifySubscriptionInvalid.Code {
		t.Fatalf("expected notify subscription error code, got %s", ack.Code)
	}
	if ack.Action != "publish" {
		t.Fatalf("unexpected ack action: %s", ack.Action)
	}
}

// TestServeWSRequiresRealtimeDependenciesWithNotifyCode 确认实时通道装配缺失时返回 M10 专属错误码。
func TestServeWSRequiresRealtimeDependenciesWithNotifyCode(t *testing.T) {
	err := ServeWS(nil, nil, nil, nil)
	if err == nil {
		t.Fatalf("expected missing realtime dependencies to fail")
	}
	if ae, ok := apperr.As(err); !ok || ae.Code != apperr.ErrNotifyRealtimeUnavailable.Code {
		t.Fatalf("expected notify realtime unavailable error, got %v", err)
	}
}
