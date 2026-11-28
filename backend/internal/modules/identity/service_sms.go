// identity service_sms 文件实现短信验证码发送、限频和一次性校验。
package identity

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"chaimir/internal/platform/ids"
	"chaimir/internal/platform/intx"
	platformredis "chaimir/internal/platform/redis"
	"chaimir/internal/platform/tenant"
	"chaimir/internal/platform/timex"
	"chaimir/pkg/apperr"
	"chaimir/pkg/crypto"
)

// SendSMS 发送短信验证码,执行同号同场景重发间隔和每日上限。
func (s *Service) SendSMS(ctx context.Context, req SendSMSRequest) (sendErr error) {
	if err := ValidatePhone(req.Phone); err != nil {
		return err
	}
	if req.Scene != SMSSceneLogin && req.Scene != SMSSceneReset && req.Scene != SMSSceneChangePhone {
		return apperr.ErrIdentitySMSSceneInvalid
	}
	phone := strings.TrimSpace(req.Phone)
	tenantID, err := s.resolveSMSSendTenant(ctx, phone, req.Scene, req.TenantID.Int64())
	if err != nil {
		return err
	}
	phoneHash, err := s.phoneHash(phone)
	if err != nil {
		return apperr.ErrInternal.WithCause(err)
	}
	// 先写短间隔限频键,避免短信网关慢响应时同一号码被并发刷爆。
	resendKey := fmt.Sprintf("identity:sms:resend:%s:%s:%d", ids.Format(tenantID), phoneHash, req.Scene)
	resendWindow := time.Duration(s.cfg.SMSResendSeconds) * time.Second
	reservation, err := s.redis.ObtainLease(ctx, resendKey, resendWindow)
	if errors.Is(err, platformredis.ErrLeaseHeld) {
		return apperr.ErrIdentitySMSTooFrequent
	}
	if err != nil {
		return apperr.ErrInternal.WithCause(err)
	}
	// 每日上限按手机号哈希统计,日志和 Redis key 都不暴露手机号明文。
	dayKey := fmt.Sprintf("identity:sms:daily:%s:%s:%s", ids.Format(tenantID), phoneHash, timex.Now().Format("20060102"))
	defer func() {
		if sendErr == nil {
			return
		}
		cleanup, cancel := context.WithTimeout(context.WithoutCancel(ctx), resendWindow)
		defer cancel()
		// 请求结果丢失时仍可退还；占用标识不会匹配其他请求。
		refundErr := s.redis.RefundQuota(cleanup, dayKey, reservation.Token())
		releaseErr := reservation.Release(cleanup)
		if errors.Is(releaseErr, platformredis.ErrLeaseNotHeld) {
			releaseErr = nil
		}
		if refundErr != nil || releaseErr != nil {
			sendErr = apperr.AsAppError(errors.Join(sendErr, refundErr, releaseErr))
		}
	}()
	allowed, err := s.redis.ReserveQuota(ctx, dayKey, reservation.Token(), s.cfg.SMSDailyLimit, 24*time.Hour)
	if err != nil {
		return apperr.ErrInternal.WithCause(err)
	}
	if !allowed {
		return apperr.ErrIdentitySMSDailyLimited
	}
	code, err := crypto.RandomToken(6)
	if err != nil {
		return apperr.ErrInternal.WithCause(err)
	}
	codeHash, err := s.hashSecret(code)
	if err != nil {
		return apperr.ErrInternal.WithCause(err)
	}
	if err := s.store.TenantTx(ctx, tenantID, func(ctx context.Context, tx TxStore) error {
		_, err := tx.CreateSMSCode(ctx, CreateSMSCodeInput{
			ID:        s.ids.Generate(),
			TenantID:  tenantID,
			PhoneHash: phoneHash,
			CodeHash:  codeHash,
			Scene:     req.Scene,
			ExpireAt:  timex.Now().Add(time.Duration(s.cfg.SMSCodeTTLMinutes) * time.Minute),
		})
		return err
	}); err != nil {
		return apperr.ErrInternal.WithCause(err)
	}
	// 先持久化哈希再发送明文验证码,避免用户收到数据库中不存在的验证码。
	if err := s.sms.Send(ctx, phone, req.Scene, code); err != nil {
		return apperr.AsAppError(err)
	}
	return nil
}

// verifySMSCodeInTx 在调用方事务内完成验证码校验和一次性消费。
func (s *Service) verifySMSCodeInTx(ctx context.Context, tx TxStore, tenantID int64, phoneHash string, scene int16, codeHash string) error {
	row, err := tx.GetLatestSMSCode(ctx, tenantID, phoneHash, scene)
	if err != nil {
		if !isNoRows(err) {
			return apperr.ErrInternal.WithCause(err)
		}
		return apperr.ErrIdentitySMSInvalid
	}
	if row.Used || !timex.Now().Before(row.ExpireAt) {
		return apperr.ErrIdentitySMSInvalid
	}
	maxAttempts, ok := intx.Int16(s.cfg.SMSVerifyMaxAttempts)
	if !ok || maxAttempts <= 0 {
		return apperr.ErrInternal
	}
	if row.VerifyAttempts >= maxAttempts {
		return apperr.ErrIdentitySMSAttemptsLimited
	}
	if !crypto.EqualHMAC(row.CodeHash, codeHash) {
		// 错误尝试必须落库计数,不能只在内存中统计,否则多实例部署会绕过上限。
		if err := tx.IncrementSMSVerifyAttempts(ctx, tenantID, row.ID); err != nil {
			return err
		}
		return apperr.ErrIdentitySMSInvalid
	}
	return tx.MarkSMSCodeUsed(ctx, tenantID, row.ID)
}

// isSMSVerificationRejection 区分应提交错误次数的认证拒绝和必须回滚的存储故障。
func isSMSVerificationRejection(err error) bool {
	return errors.Is(err, apperr.ErrIdentitySMSInvalid) || errors.Is(err, apperr.ErrIdentitySMSAttemptsLimited)
}

// smsCredentialHashes 统一计算短信验证码查询哈希和明文验证码哈希。
func (s *Service) smsCredentialHashes(phone string, code string) (string, string, error) {
	phoneHash, err := s.phoneHash(phone)
	if err != nil {
		return "", "", apperr.ErrInternal.WithCause(err)
	}
	codeHash, err := s.hashSecret(strings.TrimSpace(code))
	if err != nil {
		return "", "", apperr.ErrInternal.WithCause(err)
	}
	return phoneHash, codeHash, nil
}

// resolveSMSSendTenant 按验证码场景定位验证码所属租户。
func (s *Service) resolveSMSSendTenant(ctx context.Context, phone string, scene int16, requestedTenantID int64) (int64, error) {
	switch scene {
	case SMSSceneLogin:
		tenantID, err := s.resolveSMSCredentialTenant(ctx, phone, requestedTenantID, apperr.ErrIdentitySMSNeedsTenant, apperr.ErrIdentitySMSNeedsTenant)
		if err != nil {
			return 0, err
		}
		if err := s.ensureTenantAcceptsCredentials(ctx, tenantID); err != nil {
			return 0, err
		}
		return tenantID, nil
	case SMSSceneReset:
		tenantID, err := s.resolveSMSCredentialTenant(ctx, phone, requestedTenantID, apperr.ErrIdentityResetPasswordTenantInvalid, apperr.ErrIdentityResetPasswordTenantInvalid)
		if err != nil {
			return 0, err
		}
		if err := s.ensureTenantAcceptsCredentials(ctx, tenantID); err != nil {
			return 0, err
		}
		return tenantID, nil
	case SMSSceneChangePhone:
		id, ok := tenant.FromContext(ctx)
		if !ok {
			return 0, apperr.ErrUnauthorized
		}
		if id.IsPlatform || id.TenantID <= 0 || id.AccountID <= 0 {
			return 0, apperr.ErrForbidden
		}
		if requestedTenantID > 0 && requestedTenantID != id.TenantID {
			return 0, apperr.ErrForbidden
		}
		if err := s.ensureTenantAcceptsCredentials(ctx, id.TenantID); err != nil {
			return 0, err
		}
		return id.TenantID, nil
	default:
		return 0, apperr.ErrIdentitySMSSceneInvalid
	}
}

// ensureTenantAcceptsCredentials 统一校验短信和登录凭证所在租户仍允许认证操作。
func (s *Service) ensureTenantAcceptsCredentials(ctx context.Context, tenantID int64) error {
	if tenantID <= 0 {
		return apperr.ErrIdentitySMSNeedsTenant
	}
	if err := s.store.TenantTx(ctx, tenantID, func(ctx context.Context, tx TxStore) error {
		tenantSnapshot, err := tx.GetTenantByID(ctx, tenantID)
		if err != nil {
			return err
		}
		return EnsureTenantCanLogin(tenantSnapshot, timex.Now())
	}); err != nil {
		return apperr.AsAppError(err)
	}
	return nil
}

// resolveSMSCredentialTenant 根据手机号和可选 tenant_id 定位短信凭证归属租户。
func (s *Service) resolveSMSCredentialTenant(ctx context.Context, phone string, requestedTenantID int64, multiTenantErr *apperr.Error, mismatchErr *apperr.Error) (int64, error) {
	phoneHash, err := s.phoneHash(strings.TrimSpace(phone))
	if err != nil {
		return 0, apperr.ErrInternal.WithCause(err)
	}
	var candidates []LoginCandidate
	if err := s.store.PrivilegedTx(ctx, func(ctx context.Context, tx TxStore) error {
		rows, err := tx.ListAccountsByPhoneHash(ctx, phoneHash)
		if err != nil {
			return err
		}
		candidates = rows
		return nil
	}); err != nil {
		return 0, apperr.ErrIdentityInvalidCredentials.WithCause(err)
	}
	candidates = filterLoginCandidatesForDeployment(candidates, s.deploy)
	if len(candidates) == 0 {
		return 0, apperr.ErrIdentityInvalidCredentials
	}
	if requestedTenantID > 0 {
		for _, candidate := range candidates {
			if candidate.TenantID == requestedTenantID {
				return requestedTenantID, nil
			}
		}
		return 0, mismatchErr
	}
	if len(candidates) == 1 {
		return candidates[0].TenantID, nil
	}
	return 0, multiTenantErr
}
