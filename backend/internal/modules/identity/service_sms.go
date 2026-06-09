// M1 短信验证码服务:发送(限频)与校验。
// 依据 docs/01 §6:验证码存哈希;限频 Redis(同号 60s 一条 + 单日上限);校验限尝试次数且一次性使用。
package identity

import (
	"context"
	"crypto/rand"
	"fmt"
	"io"
	"math/big"
	"time"

	"chaimir/internal/platform/ids"
	"chaimir/internal/platform/tenant"
	"chaimir/internal/platform/timex"
	"chaimir/pkg/apperr"
	"chaimir/pkg/crypto"
)

// SendSms 发送验证码:Redis 限频 → 生成码 → 存哈希 → 经 sender 下发。
// 登录/换绑场景需先定位租户;找回(scene=2)可无租户(tenant_id 存空)。
func (s *Service) SendSms(ctx context.Context, req SendSmsRequest) error {
	if !validCNPhone(req.Phone) {
		return apperr.ErrPhoneInvalid
	}
	ph := s.phoneHash(req.Phone)

	// 限频:同号在配置窗口内只允许一条。
	gateKey := fmt.Sprintf("sms:gate:%s:%d", ph, req.Scene)
	ok, err := s.redis.SetNX(ctx, gateKey, s.smsResendInterval)
	if err != nil {
		return apperr.ErrSmsRateLimitStoreFailed.WithCause(err)
	}
	if !ok {
		return apperr.ErrSmsRateLimited
	}
	// 限频:单号单日上限。
	dayKey := fmt.Sprintf("sms:day:%s", ph)
	cnt, err := s.redis.IncrWithTTL(ctx, dayKey, 24*time.Hour)
	if err != nil {
		return apperr.ErrSmsRateLimitStoreFailed.WithCause(err)
	}
	if cnt > s.smsDailyLimit {
		return apperr.ErrSmsRateLimited
	}

	// 生成 6 位验证码,存哈希(不存明文)。
	code, err := genSmsCode()
	if err != nil {
		return apperr.ErrSmsCodeGenerateFailed.WithCause(err)
	}
	codeHash := crypto.HMACHash(s.hmacKey, code)

	// 定位验证码所属租户:换绑来自当前登录态;登录按手机号定位;找回允许无租户。
	tenantID, hasTenant, err := smsTenantIDFromContext(ctx, req.Scene)
	if err != nil {
		return err
	}
	if !hasTenant && req.Scene == SmsSceneLogin {
		accts, e := s.repo.findAccountTenantCandidatesByPhone(ctx, ph)
		if e != nil {
			return e
		}
		tenantID, e = selectLoginSmsTenantID(accts, req.TenantID)
		if e != nil {
			return e
		}
	}

	// 验证码记录写入(找回场景 tenant_id 可空 → 用特权连接写,避免 RLS 拦截)。
	if err := s.repo.createSmsCode(ctx, tenantID, s.idgen.Generate(), ph, codeHash, req.Scene, timex.Now().Add(s.smsCodeTTL)); err != nil {
		return apperr.ErrSmsCodeStoreFailed.WithCause(err)
	}

	// 下发验证码(经 sender;dev sender 记日志,生产接真实网关)。
	if err := s.sms.Send(ctx, req.Phone, code, req.Scene); err != nil {
		return apperr.ErrSmsSendFailed.WithCause(err)
	}
	return nil
}

// verifySmsCode 校验验证码:取最新未用且未过期的码,错误尝试超限即失效,比对成功后标记已用。
func (s *Service) verifySmsCode(ctx context.Context, tenantID int64, phoneHash string, scene int16, code string) error {
	codeHash := crypto.HMACHash(s.hmacKey, code)
	row, err := s.repo.getLatestSmsCode(ctx, tenantID, phoneHash, scene)
	if err != nil {
		if ae, ok := apperr.As(err); ok {
			return ae
		}
		return apperr.ErrSmsVerifyStoreFailed.WithCause(err)
	}
	if row.CodeHash != codeHash {
		attempts, err := s.recordSmsVerificationFailure(ctx, row)
		if err != nil {
			return err
		}
		if smsVerificationAttemptsExceeded(attempts, s.smsVerifyMaxAttempts) {
			if err := s.repo.markSmsCodeUsed(ctx, tenantID, row.ID); err != nil {
				return apperr.ErrSmsVerifyStoreFailed.WithCause(err)
			}
			return apperr.ErrSmsCodeAttemptsExceeded
		}
		return apperr.ErrSmsCodeInvalid
	}
	if err := s.repo.markSmsCodeUsed(ctx, tenantID, row.ID); err != nil {
		return apperr.ErrSmsVerifyStoreFailed.WithCause(err)
	}
	return nil
}

// recordSmsVerificationFailure 记录单个验证码的校验失败次数,计数窗口不超过验证码剩余有效期。
func (s *Service) recordSmsVerificationFailure(ctx context.Context, row SmsCodeSnapshot) (int64, error) {
	if s.smsVerifyMaxAttempts <= 0 {
		return 0, nil
	}
	if s.redis == nil {
		return 0, apperr.ErrSmsVerifyStoreFailed.WithCause(fmt.Errorf("验证码校验尝试计数需要 Redis"))
	}
	ttl := row.ExpireAt.Sub(timex.Now())
	if ttl <= 0 {
		ttl = time.Minute
	}
	attempts, err := s.redis.IncrWithTTL(ctx, smsVerificationAttemptKey(row.ID), ttl)
	if err != nil {
		return 0, apperr.ErrSmsVerifyStoreFailed.WithCause(err)
	}
	return attempts, nil
}

// smsVerificationAttemptsExceeded 判断当前错误次数是否达到验证码失效阈值。
func smsVerificationAttemptsExceeded(attempts, maxAttempts int64) bool {
	return maxAttempts > 0 && attempts >= maxAttempts
}

// smsVerificationAttemptKey 构造验证码校验尝试计数键。
func smsVerificationAttemptKey(codeID int64) string {
	return fmt.Sprintf("sms:verify:%d", codeID)
}

// smsTenantIDFromContext 按验证码场景解析租户归属。
func smsTenantIDFromContext(ctx context.Context, scene int16) (int64, bool, error) {
	switch scene {
	case SmsSceneLogin, SmsSceneReset:
		return 0, false, nil
	case SmsSceneRebind:
		id, ok := tenant.FromContext(ctx)
		if !ok || id.TenantID == 0 {
			return 0, false, apperr.ErrUnauthorized
		}
		return id.TenantID, true, nil
	default:
		return 0, false, apperr.ErrSmsCodeInvalid
	}
}

// selectLoginSmsTenantID 根据手机号候选账号和用户选择决定验证码写入租户。
func selectLoginSmsTenantID(accts []AccountTenantCandidate, reqTenantID string) (int64, error) {
	if len(accts) == 0 {
		return 0, nil
	}
	if len(accts) == 1 {
		return accts[0].TenantID, nil
	}
	if reqTenantID == "" {
		return 0, apperr.ErrSmsTenantRequired
	}
	tid, ok := ids.Parse(reqTenantID)
	if !ok {
		return 0, apperr.ErrSmsTenantInvalid
	}
	for _, acct := range accts {
		if acct.TenantID == tid {
			return tid, nil
		}
	}
	return 0, apperr.ErrSmsTenantInvalid
}

// genSmsCode 生成 6 位数字验证码(加密随机)。
func genSmsCode() (string, error) {
	return genSmsCodeFromReader(rand.Reader)
}

// genSmsCodeFromReader 从指定随机源生成验证码,便于测试随机源错误路径。
func genSmsCodeFromReader(reader io.Reader) (string, error) {
	n, err := rand.Int(reader, big.NewInt(1000000))
	if err != nil {
		return "", fmt.Errorf("生成短信验证码失败: %w", err)
	}
	return fmt.Sprintf("%06d", n.Int64()), nil
}
