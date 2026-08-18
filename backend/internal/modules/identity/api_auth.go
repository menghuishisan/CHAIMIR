// identity api_auth 文件承接认证类 HTTP 请求并委托 service。
package identity

import (
	"time"

	"chaimir/internal/platform/auth"
	"chaimir/internal/platform/httpx"
	"chaimir/internal/platform/tenant"
	"chaimir/internal/platform/timex"
	"chaimir/pkg/apperr"

	"github.com/gin-gonic/gin"
)

// authAPI 封装认证 HTTP handler 依赖,避免匿名函数承载核心入口职责。
type authAPI struct {
	svc   *Service
	authn *auth.Manager
}

// registerAuthRoutes 注册登录、刷新、短信、激活、找回密码、SSO 入口和登出路由。
func registerAuthRoutes(r gin.IRouter, svc *Service, authn *auth.Manager) {
	api := authAPI{svc: svc, authn: authn}
	g := r.Group("/auth")
	rateLimit := httpx.RateLimitMiddleware(svc.redis, "identity:auth-rate", svc.cfg.AuthRateMax, time.Duration(svc.cfg.AuthRateWindowSeconds)*time.Second)
	g.POST("/login/platform", rateLimit, api.loginPlatform)
	g.POST("/login/phone", rateLimit, api.loginPhone)
	g.POST("/login/no", rateLimit, api.loginNo)
	g.POST("/login/sms", rateLimit, api.loginSMS)
	g.POST("/sms/send", rateLimit, api.sendSMS)
	g.POST("/refresh", rateLimit, api.refreshToken)
	g.POST("/ws-ticket", authn.Middleware(), api.issueWebSocketTicket)
	g.POST("/browser-ticket", authn.Middleware(), api.issueBrowserAccessTicket)
	g.POST("/password/reset", rateLimit, api.resetPassword)
	g.POST("/activate", rateLimit, api.activate)
	g.POST("/logout", authn.Middleware(), api.logout)
	ssoRateLimit := httpx.RateLimitMiddleware(svc.redis, "identity:sso-rate", svc.cfg.AuthRateMax, time.Duration(svc.cfg.AuthRateWindowSeconds)*time.Second)
	g.GET("/sso/:tenant_code/login", ssoRateLimit, api.casLoginURL)
	g.GET("/sso/:tenant_code/callback", ssoRateLimit, api.casCallback)
	g.POST("/sso/:tenant_code/ldap", rateLimit, api.ldapLogin)
}

// loginPlatform 绑定平台管理员登录请求并返回平台级 token。
func (a authAPI) loginPlatform(c *gin.Context) {
	var req LoginPlatformRequest
	if !httpx.BindJSON(c, &req) {
		return
	}
	out, err := a.svc.LoginPlatform(c.Request.Context(), req, c.GetHeader("User-Agent"), c.ClientIP())
	if err != nil {
		httpx.Write(c, struct{}{}, err)
		return
	}
	a.writeLoginResponse(c, out, req.Remember)
}

// loginPhone 绑定手机号密码登录请求,一号多校时由 service 返回租户选择结果。
func (a authAPI) loginPhone(c *gin.Context) {
	var req LoginPhoneRequest
	if !httpx.BindJSON(c, &req) {
		return
	}
	out, err := a.svc.LoginPhone(c.Request.Context(), req, c.GetHeader("User-Agent"), c.ClientIP())
	if err != nil {
		httpx.Write(c, struct{}{}, err)
		return
	}
	a.writeLoginResponse(c, out, req.Remember)
}

// loginNo 绑定学校短码加学号工号的备用登录请求。
func (a authAPI) loginNo(c *gin.Context) {
	var req LoginNoRequest
	if !httpx.BindJSON(c, &req) {
		return
	}
	out, err := a.svc.LoginNo(c.Request.Context(), req, c.GetHeader("User-Agent"), c.ClientIP())
	if err != nil {
		httpx.Write(c, struct{}{}, err)
		return
	}
	a.writeLoginResponse(c, out, req.Remember)
}

// loginSMS 绑定短信验证码登录请求,验证码校验和会话签发均由 service 完成。
func (a authAPI) loginSMS(c *gin.Context) {
	var req LoginSMSRequest
	if !httpx.BindJSON(c, &req) {
		return
	}
	out, err := a.svc.LoginSMS(c.Request.Context(), req, c.GetHeader("User-Agent"), c.ClientIP())
	if err != nil {
		httpx.Write(c, struct{}{}, err)
		return
	}
	a.writeLoginResponse(c, out, req.Remember)
}

// sendSMS 绑定发送验证码请求,API 层只读取参数不执行限频逻辑。
func (a authAPI) sendSMS(c *gin.Context) {
	var req SendSMSRequest
	if !httpx.BindJSON(c, &req) {
		return
	}
	if req.Scene == SMSSceneChangePhone && !a.authn.AuthenticateAccess(c) {
		return
	}
	if err := a.svc.SendSMS(c.Request.Context(), req); err != nil {
		httpx.Write(c, struct{}{}, err)
		return
	}
	httpx.Write(c, struct{}{}, nil)
}

// refreshToken 从 HttpOnly cookie 读取并轮转 Refresh Token，令牌不接受 JSON 输入。
func (a authAPI) refreshToken(c *gin.Context) {
	if !auth.ValidRefreshRequestHeader(c.GetHeader(auth.RefreshRequestHeader)) {
		httpx.Write(c, struct{}{}, apperr.ErrIdentitySessionInvalid)
		return
	}
	refreshToken, persistent, ok := auth.RefreshCookieFromRequest(c)
	if !ok {
		httpx.Write(c, struct{}{}, apperr.ErrIdentitySessionInvalid)
		return
	}
	out, err := a.svc.RefreshToken(c.Request.Context(), refreshToken, c.GetHeader("User-Agent"), c.ClientIP())
	if err != nil {
		a.authn.ClearRefreshCookie(c)
		httpx.Write(c, struct{}{}, err)
		return
	}
	a.writeLoginResponse(c, out, persistent)
}

// issueWebSocketTicket 用当前服务端会话为指定实时通道签发短时连接票据。
func (a authAPI) issueWebSocketTicket(c *gin.Context) {
	a.issuePathTicket(c, a.authn.IssueWebSocketTicket)
}

// issueBrowserAccessTicket 用当前服务端会话为浏览器工具路径前缀签发短时入口票据。
func (a authAPI) issueBrowserAccessTicket(c *gin.Context) {
	a.issuePathTicket(c, a.authn.IssueBrowserAccessTicket)
}

// issuePathTicket 统一绑定路径票据请求与当前服务端会话,具体 token 类型由 auth 签发器决定。
func (a authAPI) issuePathTicket(c *gin.Context, issue func(auth.SessionIdentity) (string, time.Time, error)) {
	var req PathTicketRequest
	if !httpx.BindJSON(c, &req) {
		return
	}
	id, ok := tenant.FromContext(c.Request.Context())
	if !ok {
		httpx.Write(c, struct{}{}, apperr.ErrIdentitySessionContextMissing)
		return
	}
	sessionID, ok := currentSessionID(c)
	if !ok {
		return
	}
	ticket, expiresAt, err := issue(auth.SessionIdentity{
		TenantID:   id.TenantID,
		AccountID:  id.AccountID,
		SessionID:  sessionID,
		IsPlatform: id.IsPlatform,
		Path:       req.Path,
	})
	if err != nil {
		httpx.Write(c, struct{}{}, apperr.ErrUnauthorized.WithCause(err))
		return
	}
	httpx.Write(c, PathTicketResponse{Ticket: ticket, ExpiresAt: timex.RFC3339OrEmpty(expiresAt)}, nil)
}

// resetPassword 绑定找回密码请求,短信校验和密码更新由 service 原子处理。
func (a authAPI) resetPassword(c *gin.Context) {
	var req PasswordResetRequest
	if !httpx.BindJSON(c, &req) {
		return
	}
	if err := a.svc.ResetPassword(c.Request.Context(), req); err != nil {
		httpx.Write(c, struct{}{}, err)
		return
	}
	httpx.Write(c, struct{}{}, nil)
}

// activate 绑定激活码开通请求,激活码明文只进入 service 校验不落库。
func (a authAPI) activate(c *gin.Context) {
	var req ActivateRequest
	if !httpx.BindJSON(c, &req) {
		return
	}
	out, err := a.svc.Activate(c.Request.Context(), req)
	if err != nil {
		httpx.Write(c, struct{}{}, err)
		return
	}
	a.writeLoginResponse(c, out, false)
}

// logout 吊销当前 JWT 对应的服务端会话。
func (a authAPI) logout(c *gin.Context) {
	id, ok := currentSessionID(c)
	if !ok {
		return
	}
	if err := a.svc.Logout(c.Request.Context(), id); err != nil {
		httpx.Write(c, struct{}{}, err)
		return
	}
	a.authn.ClearRefreshCookie(c)
	httpx.Write(c, struct{}{}, nil)
}

// casLoginURL 生成 CAS 登录跳转地址,回调 origin 白名单校验由 service 执行。
func (a authAPI) casLoginURL(c *gin.Context) {
	out, err := a.svc.CASLoginURL(c.Request.Context(), c.Param("tenant_code"), c.Query("service"))
	if err != nil {
		httpx.Write(c, struct{}{}, err)
		return
	}
	httpx.Write(c, CASLoginURLResponse{RedirectURL: out}, nil)
}

// casCallback 绑定 CAS 回调参数并委托 service 完成验票与名单匹配。
func (a authAPI) casCallback(c *gin.Context) {
	out, err := a.svc.CASCallback(c.Request.Context(), c.Param("tenant_code"), c.Query("ticket"), c.Query("service"), c.GetHeader("User-Agent"), c.ClientIP())
	if err != nil {
		httpx.Write(c, struct{}{}, err)
		return
	}
	a.writeLoginResponse(c, out, false)
}

// ldapLogin 绑定 LDAP 登录请求,实际目录绑定与名单匹配由 service 完成。
func (a authAPI) ldapLogin(c *gin.Context) {
	var req LDAPLoginRequest
	if !httpx.BindJSON(c, &req) {
		return
	}
	out, err := a.svc.LDAPLogin(c.Request.Context(), c.Param("tenant_code"), req, c.GetHeader("User-Agent"), c.ClientIP())
	if err != nil {
		httpx.Write(c, struct{}{}, err)
		return
	}
	a.writeLoginResponse(c, out, false)
}

// writeLoginResponse 把 refresh token 固定写入 HttpOnly cookie，响应体仅保留短期 access token 和账号状态。
func (a authAPI) writeLoginResponse(c *gin.Context, out LoginResponse, persistent bool) {
	if out.RefreshToken != "" {
		a.authn.SetRefreshCookie(c, out.RefreshToken, persistent)
	}
	httpx.Write(c, out, nil)
}
