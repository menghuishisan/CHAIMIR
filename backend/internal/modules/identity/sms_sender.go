// 短信下发能力(SMS sender)。
// 依据 CLAUDE.md §4 的生产级要求:定义接口由部署注入真实网关;
//
//	开发环境用 LogSmsSender(仅记日志,绝不用于生产 —— 通过配置选择)。
//
// 真实网关(阿里云/腾讯云短信)作为独立实现接入,不在 M1 内置具体厂商 SDK。
package identity

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"chaimir/internal/platform/netx"
)

// SmsSender 是短信下发能力契约。
type SmsSender interface {
	// Send 向手机号下发验证码;scene 用于选择短信模板。
	Send(ctx context.Context, phone, code string, scene int16) error
}

// LogSmsSender 是开发用 sender:把验证码记到日志(脱敏手机号),不真实发送。
// ⚠ 仅限非生产环境;生产必须注入真实网关 sender(装配时按 APP_ENV 选择)。
type LogSmsSender struct{}

// Send 记录验证码到日志(开发用)。
func (LogSmsSender) Send(ctx context.Context, phone, code string, scene int16) error {
	slog.WarnContext(ctx, "开发模式短信(未真实发送)",
		slog.String("phone", maskPhone(phone)),
		slog.Int("scene", int(scene)),
		slog.String("code", code), // 开发可见;生产 sender 不记明文。
	)
	return nil
}

// HTTPSmsConfig 是通用 HTTP 短信网关配置,由部署侧接入具体国内服务商代理。
type HTTPSmsConfig struct {
	Endpoint       string
	Token          string
	LoginTemplate  string
	ResetTemplate  string
	ChangeTemplate string
	Timeout        time.Duration
}

// HTTPSmsSender 通过受控 HTTP 网关发送验证码,避免在业务代码绑定具体厂商 SDK。
type HTTPSmsSender struct {
	cfg    HTTPSmsConfig
	client *http.Client
}

// NewHTTPSmsSender 构造 HTTP 短信发送器,真实网关超时必须由配置显式注入。
func NewHTTPSmsSender(cfg HTTPSmsConfig) (*HTTPSmsSender, error) {
	if cfg.Timeout <= 0 {
		return nil, fmt.Errorf("短信网关超时时间必须大于 0")
	}
	endpoint, err := netx.ValidatePublicHTTPURL(cfg.Endpoint)
	if err != nil {
		return nil, fmt.Errorf("短信网关地址非法: %w", err)
	}
	cfg.Endpoint = endpoint
	return &HTTPSmsSender{
		cfg:    cfg,
		client: netx.NewPublicHTTPClient(cfg.Timeout),
	}, nil
}

// Send 调用 HTTP 网关发送验证码;失败状态码返回错误并保留状态上下文。
func (s *HTTPSmsSender) Send(ctx context.Context, phone, code string, scene int16) (err error) {
	template, err := s.templateForScene(scene)
	if err != nil {
		return err
	}
	payload := map[string]any{
		"phone":    phone,
		"code":     code,
		"scene":    scene,
		"template": template,
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("短信请求序列化失败: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, s.cfg.Endpoint, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("创建短信请求失败: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	if s.cfg.Token != "" {
		req.Header.Set("Authorization", "Bearer "+s.cfg.Token)
	}

	resp, err := s.client.Do(req)
	if err != nil {
		return fmt.Errorf("调用短信网关失败: %w", err)
	}
	defer func() {
		if closeErr := resp.Body.Close(); closeErr != nil {
			err = errors.Join(err, fmt.Errorf("关闭短信网关响应失败: %w", closeErr))
		}
	}()
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return fmt.Errorf("短信网关返回异常状态: %d", resp.StatusCode)
	}
	return nil
}

// templateForScene 选择验证码场景对应的短信模板。
func (s *HTTPSmsSender) templateForScene(scene int16) (string, error) {
	var template string
	switch scene {
	case SmsSceneLogin:
		template = s.cfg.LoginTemplate
	case SmsSceneReset:
		template = s.cfg.ResetTemplate
	case SmsSceneRebind:
		template = s.cfg.ChangeTemplate
	default:
		return "", fmt.Errorf("不支持的短信场景: %d", scene)
	}
	if strings.TrimSpace(template) == "" {
		return "", fmt.Errorf("短信场景 %d 未配置模板", scene)
	}
	return template, nil
}
