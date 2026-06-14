// identity service_sms_sender 文件封装短信网关调用,验证码明文只存在于发送路径且不写日志。
package identity

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	"chaimir/internal/platform/config"
	"chaimir/internal/platform/jsonx"
	"chaimir/internal/platform/netx"
	"chaimir/pkg/apperr"
	"chaimir/pkg/logging"
)

// SMSSender 是短信发送能力契约,便于 service 只依赖明确边界。
type SMSSender interface {
	// Send 发送指定场景的验证码短信,不得记录验证码明文。
	Send(ctx context.Context, phone string, scene int16, code string) error
}

// HTTPSMSSender 通过受控 HTTP 短信代理网关发送验证码。
type HTTPSMSSender struct {
	cfg    config.SMSConfig
	client *http.Client
}

// NewSMSSender 根据统一配置创建短信发送器。
func NewSMSSender(cfg config.SMSConfig) SMSSender {
	return &HTTPSMSSender{
		cfg:    cfg,
		client: netx.NewPublicHTTPClient(time.Duration(cfg.TimeoutSeconds) * time.Second),
	}
}

// Send 按配置发送验证码;log provider 仅允许开发环境显式使用,生产应配置 HTTP 网关。
func (s *HTTPSMSSender) Send(ctx context.Context, phone string, scene int16, code string) error {
	switch strings.ToLower(strings.TrimSpace(s.cfg.Provider)) {
	case "log":
		return nil
	case "http":
		return s.sendHTTP(ctx, phone, scene, code)
	default:
		return fmt.Errorf("不支持的短信服务商配置: %s", s.cfg.Provider)
	}
}

// sendHTTP 调用统一短信代理网关,避免模块直接耦合具体厂商 SDK。
func (s *HTTPSMSSender) sendHTTP(ctx context.Context, phone string, scene int16, code string) error {
	if strings.TrimSpace(s.cfg.Endpoint) == "" || strings.TrimSpace(s.cfg.Token) == "" {
		return fmt.Errorf("短信 HTTP 网关配置不完整")
	}
	endpoint, err := netx.ValidatePublicHTTPURL(s.cfg.Endpoint)
	if err != nil {
		return fmt.Errorf("短信 HTTP 网关地址不安全: %w", err)
	}
	template := s.template(scene)
	if template == "" {
		return fmt.Errorf("短信模板配置不完整")
	}
	body, err := jsonx.AnyBytes(map[string]string{
		"phone":    phone,
		"template": template,
		"code":     code,
	}, apperr.ErrInternal)
	if err != nil {
		return fmt.Errorf("序列化短信请求失败: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("创建短信请求失败: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+s.cfg.Token)
	resp, err := s.client.Do(req)
	if err != nil {
		return fmt.Errorf("调用短信网关失败: %w", err)
	}
	defer logging.CloseContext(ctx, "关闭短信网关响应失败", resp.Body)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("短信网关返回异常状态: %d", resp.StatusCode)
	}
	return nil
}

// template 根据验证码场景选择短信模板。
func (s *HTTPSMSSender) template(scene int16) string {
	switch scene {
	case SMSSceneLogin:
		return s.cfg.LoginTemplate
	case SMSSceneReset:
		return s.cfg.ResetTemplate
	case SMSSceneChangePhone:
		return s.cfg.ChangeTemplate
	default:
		return ""
	}
}
