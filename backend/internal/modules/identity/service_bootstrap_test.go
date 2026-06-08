// M1 私有化初始化输入边界测试。
package identity

import (
	"context"
	"testing"

	"chaimir/pkg/apperr"
)

// TestBootstrapPrivateSchoolValidatesConfig 确认初始化参数缺失时返回 M1 专属错误码。
func TestBootstrapPrivateSchoolValidatesConfig(t *testing.T) {
	svc := &Service{}
	_, err := svc.BootstrapPrivateSchool(context.Background(), BootstrapPrivateSchoolRequest{
		TenantID:  1,
		Code:      "school-demo",
		Name:      "示例学校",
		Type:      SchoolTypeCollege,
		Phone:     "13800138000",
		AdminName: "首个管理员",
	})
	if err != apperr.ErrBootstrapConfigInvalid {
		t.Fatalf("expected ErrBootstrapConfigInvalid, got %v", err)
	}
}

// TestBootstrapPrivateSchoolRejectsWeakInitialPassword 确认初始化管理员密码复用统一密码强度策略。
func TestBootstrapPrivateSchoolRejectsWeakInitialPassword(t *testing.T) {
	svc := &Service{}
	_, err := svc.BootstrapPrivateSchool(context.Background(), BootstrapPrivateSchoolRequest{
		TenantID:  1,
		Code:      "school-demo",
		Name:      "示例学校",
		Type:      SchoolTypeCollege,
		Phone:     "13800138000",
		AdminName: "首个管理员",
		Password:  "Password123",
	})
	if err != apperr.ErrWeakPassword {
		t.Fatalf("expected ErrWeakPassword, got %v", err)
	}
}

// TestBootstrapPlatformAdminRejectsWeakPassword 确认平台管理员初始化也复用统一密码强度策略。
func TestBootstrapPlatformAdminRejectsWeakPassword(t *testing.T) {
	svc := &Service{}
	_, err := svc.BootstrapPlatformAdmin(context.Background(), BootstrapPlatformAdminRequest{
		Username: "platform-admin",
		Name:     "平台管理员",
		Password: "Admin123456",
	})
	if err != apperr.ErrWeakPassword {
		t.Fatalf("expected ErrWeakPassword, got %v", err)
	}
}
