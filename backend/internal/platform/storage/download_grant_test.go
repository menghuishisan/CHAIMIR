// storage_test 校验统一文件服务的受控对象访问、租户边界与下载授权模型。
package storage

import (
	"context"
	"strings"
	"testing"
	"time"
)

// TestBuildDownloadGrantRejectsUnsafeRef 确认下载授权不会接受非法对象引用。
func TestBuildDownloadGrantRejectsUnsafeRef(t *testing.T) {
	_, err := BuildDownloadGrant(context.Background(), DownloadGrantRequest{
		TenantID:     1,
		AccountID:    2,
		ObjectRef:    "https://example.com/a.txt",
		Module:       "grade",
		ResourceType: "transcript",
		ResourceID:   "1001",
		ExpiresAt:    time.Now().Add(time.Minute),
	})
	if err == nil {
		t.Fatalf("expected invalid object ref to fail")
	}
}

// TestBuildDownloadGrantRequiresTenantScopedKeyPrefix 确认下载授权要求对象路径符合统一租户前缀。
func TestBuildDownloadGrantRequiresTenantScopedKeyPrefix(t *testing.T) {
	_, err := BuildDownloadGrant(context.Background(), DownloadGrantRequest{
		TenantID:     42,
		AccountID:    2,
		ObjectRef:    "minio://chaimir-report/7/grade/transcript/1001.pdf",
		Module:       "grade",
		ResourceType: "transcript",
		ResourceID:   "1001.pdf",
		ExpiresAt:    time.Now().Add(time.Minute),
	})
	if err == nil {
		t.Fatalf("expected mismatched tenant prefix to fail")
	}
}

// TestBuildDownloadGrantBuildsAuthorizedGrant 确认统一文件服务能生成受控下载授权。
func TestBuildDownloadGrantBuildsAuthorizedGrant(t *testing.T) {
	grant, err := BuildDownloadGrant(context.Background(), DownloadGrantRequest{
		TenantID:     42,
		AccountID:    1001,
		ObjectRef:    "minio://chaimir-report/42/grade/transcript/1001.pdf",
		Module:       "grade",
		ResourceType: "transcript",
		ResourceID:   "1001.pdf",
		ExpiresAt:    time.Now().Add(5 * time.Minute),
	})
	if err != nil {
		t.Fatalf("build download grant: %v", err)
	}
	if grant.Object.Bucket != "chaimir-report" {
		t.Fatalf("bucket = %q, want chaimir-report", grant.Object.Bucket)
	}
	if grant.TenantID != 42 || grant.AccountID != 1001 {
		t.Fatalf("unexpected grant identity: %+v", grant)
	}
}

// TestSignAndVerifyDownloadGrantToken 确认统一文件服务会把下载授权签名为可校验且可还原的令牌。
func TestSignAndVerifyDownloadGrantToken(t *testing.T) {
	grant, err := BuildDownloadGrant(context.Background(), DownloadGrantRequest{
		TenantID:     42,
		AccountID:    1001,
		ObjectRef:    "minio://chaimir-report/42/grade/transcript/1001.pdf",
		Module:       "grade",
		ResourceType: "transcript",
		ResourceID:   "1001.pdf",
		ExpiresAt:    time.Now().Add(5 * time.Minute),
	})
	if err != nil {
		t.Fatalf("build download grant: %v", err)
	}

	token, err := SignDownloadGrantToken(grant, strings.Repeat("k", 32))
	if err != nil {
		t.Fatalf("sign download grant: %v", err)
	}

	verified, err := VerifyDownloadGrantToken(token, strings.Repeat("k", 32))
	if err != nil {
		t.Fatalf("verify download grant: %v", err)
	}
	if verified != grant {
		t.Fatalf("verified grant mismatch: got %+v want %+v", verified, grant)
	}
}

// TestVerifyDownloadGrantTokenRejectsTampering 确认下载授权令牌被篡改后会因签名失配被拒绝。
func TestVerifyDownloadGrantTokenRejectsTampering(t *testing.T) {
	grant, err := BuildDownloadGrant(context.Background(), DownloadGrantRequest{
		TenantID:     42,
		AccountID:    1001,
		ObjectRef:    "minio://chaimir-report/42/grade/transcript/1001.pdf",
		Module:       "grade",
		ResourceType: "transcript",
		ResourceID:   "1001.pdf",
		ExpiresAt:    time.Now().Add(5 * time.Minute),
	})
	if err != nil {
		t.Fatalf("build download grant: %v", err)
	}

	token, err := SignDownloadGrantToken(grant, strings.Repeat("k", 32))
	if err != nil {
		t.Fatalf("sign download grant: %v", err)
	}
	tampered := token[:len(token)-1] + "x"

	if _, err := VerifyDownloadGrantToken(tampered, strings.Repeat("k", 32)); err == nil {
		t.Fatalf("tampered token must be rejected")
	}
}

// TestVerifyDownloadGrantTokenRejectsExpiredGrant 确认下载授权令牌过期后不会继续放行对象下载。
func TestVerifyDownloadGrantTokenRejectsExpiredGrant(t *testing.T) {
	grant := DownloadGrant{
		TenantID:     42,
		AccountID:    1001,
		Module:       "grade",
		ResourceType: "transcript",
		ResourceID:   "1001.pdf",
		Object: ObjectRef{
			Bucket: "chaimir-report",
			Key:    "42/grade/transcript/1001.pdf",
		},
		ExpiresAt: time.Now().Add(2 * time.Minute).UTC(),
	}

	token, err := SignDownloadGrantToken(grant, strings.Repeat("k", 32))
	if err != nil {
		t.Fatalf("sign download grant: %v", err)
	}

	if _, err := verifyDownloadGrantTokenAt(token, strings.Repeat("k", 32), time.Now().Add(3*time.Minute)); err == nil {
		t.Fatalf("expired token must be rejected")
	}
}
