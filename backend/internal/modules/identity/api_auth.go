// identity api_auth 文件承接认证类 HTTP 请求并委托 service。
package identity

import (
	"chaimir/internal/platform/auth"
	"chaimir/internal/platform/httpx"

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
	g.POST("/login/platform", api.loginPlatform)
	g.POST("/login/phone", api.loginPhone)
	g.POST("/login/no", api.loginNo)
	g.POST("/login/sms", api.loginSMS)
	g.POST("/sms/send", api.sendSMS)
	g.POST("/refresh", api.refreshToken)
	g.POST("/password/reset", api.resetPassword)
	g.POST("/activate", api.activate)
	g.POST("/logout", authn.Middleware(), api.logout)
	g.GET("/sso/:tenant_code/login", api.casLoginURL)
	g.GET("/sso/:tenant_code/callback", api.casCallback)
	g.POST("/sso/:tenant_code/ldap", api.ldapLogin)
}

// loginPlatform 绑定平台管理员登录请求并返回平台级 token。
func (a authAPI) loginPlatform(c *gin.Context) {
	var req LoginPlatformRequest
	if !httpx.BindJSON(c, &req) {
		return
	}
	out, err := a.svc.LoginPlatform(c.Request.Context(), req, c.GetHeader("User-Agent"), c.ClientIP())
	if err != nil {
		httpx.Write(c, gin.H{}, err)
		return
	}
	httpx.Write(c, out, nil)
}

// loginPhone 绑定手机号密码登录请求,一号多校时由 service 返回租户选择结果。
func (a authAPI) loginPhone(c *gin.Context) {
	var req LoginPhoneRequest
	if !httpx.BindJSON(c, &req) {
		return
	}
	out, err := a.svc.LoginPhone(c.Request.Context(), req, c.GetHeader("User-Agent"), c.ClientIP())
	if err != nil {
		httpx.Write(c, gin.H{}, err)
		return
	}
	httpx.Write(c, out, nil)
}

// loginNo 绑定学校短码加学号工号的备用登录请求。
func (a authAPI) loginNo(c *gin.Context) {
	var req LoginNoRequest
	if !httpx.BindJSON(c, &req) {
		return
	}
	out, err := a.svc.LoginNo(c.Request.Context(), req, c.GetHeader("User-Agent"), c.ClientIP())
	if err != nil {
		httpx.Write(c, gin.H{}, err)
		return
	}
	httpx.Write(c, out, nil)
}

// loginSMS 绑定短信验证码登录请求,验证码校验和会话签发均由 service 完成。
func (a authAPI) loginSMS(c *gin.Context) {
	var req LoginSMSRequest
	if !httpx.BindJSON(c, &req) {
		return
	}
	out, err := a.svc.LoginSMS(c.Request.Context(), req, c.GetHeader("User-Agent"), c.ClientIP())
	if err != nil {
		httpx.Write(c, gin.H{}, err)
		return
	}
	httpx.Write(c, out, nil)
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
		httpx.Write(c, gin.H{}, err)
		return
	}
	httpx.Write(c, gin.H{}, nil)
}

// refreshToken 绑定 Refresh Token 轮转请求。
func (a authAPI) refreshToken(c *gin.Context) {
	var req RefreshRequest
	if !httpx.BindJSON(c, &req) {
		return
	}
	out, err := a.svc.RefreshToken(c.Request.Context(), req, c.GetHeader("User-Agent"), c.ClientIP())
	if err != nil {
		httpx.Write(c, gin.H{}, err)
		return
	}
	httpx.Write(c, out, nil)
}

// resetPassword 绑定找回密码请求,短信校验和密码更新由 service 原子处理。
func (a authAPI) resetPassword(c *gin.Context) {
	var req PasswordResetRequest
	if !httpx.BindJSON(c, &req) {
		return
	}
	if err := a.svc.ResetPassword(c.Request.Context(), req); err != nil {
		httpx.Write(c, gin.H{}, err)
		return
	}
	httpx.Write(c, gin.H{}, nil)
}

// activate 绑定激活码开通请求,激活码明文只进入 service 校验不落库。
func (a authAPI) activate(c *gin.Context) {
	var req ActivateRequest
	if !httpx.BindJSON(c, &req) {
		return
	}
	out, err := a.svc.Activate(c.Request.Context(), req)
	if err != nil {
		httpx.Write(c, gin.H{}, err)
		return
	}
	httpx.Write(c, out, nil)
}

// logout 吊销当前 JWT 对应的服务端会话。
func (a authAPI) logout(c *gin.Context) {
	id, ok := currentSessionID(c)
	if !ok {
		return
	}
	if err := a.svc.Logout(c.Request.Context(), id); err != nil {
		httpx.Write(c, gin.H{}, err)
		return
	}
	httpx.Write(c, gin.H{}, nil)
}

// casLoginURL 生成 CAS 登录跳转地址,回调 origin 白名单校验由 service 执行。
func (a authAPI) casLoginURL(c *gin.Context) {
	out, err := a.svc.CASLoginURL(c.Request.Context(), c.Param("tenant_code"), c.Query("service"))
	if err != nil {
		httpx.Write(c, gin.H{}, err)
		return
	}
	httpx.Write(c, gin.H{"redirect_url": out}, nil)
}

// casCallback 绑定 CAS 回调参数并委托 service 完成验票与名单匹配。
func (a authAPI) casCallback(c *gin.Context) {
	out, err := a.svc.CASCallback(c.Request.Context(), c.Param("tenant_code"), c.Query("ticket"), c.Query("service"), c.GetHeader("User-Agent"), c.ClientIP())
	if err != nil {
		httpx.Write(c, gin.H{}, err)
		return
	}
	httpx.Write(c, out, nil)
}

// ldapLogin 绑定 LDAP 登录请求,实际目录绑定与名单匹配由 service 完成。
func (a authAPI) ldapLogin(c *gin.Context) {
	var req LDAPLoginRequest
	if !httpx.BindJSON(c, &req) {
		return
	}
	out, err := a.svc.LDAPLogin(c.Request.Context(), c.Param("tenant_code"), req, c.GetHeader("User-Agent"), c.ClientIP())
	if err != nil {
		httpx.Write(c, gin.H{}, err)
		return
	}
	httpx.Write(c, out, nil)
}
