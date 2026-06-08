// M1 认证 HTTP 处理器。
package identity

import (
	"chaimir/internal/platform/httpx"
	"chaimir/pkg/apperr"
	"chaimir/pkg/response"

	"github.com/gin-gonic/gin"
)

// loginPlatform 平台管理员登录。
func (a *API) loginPlatform(c *gin.Context) {
	var req PlatformLoginRequest
	if !httpx.BindJSONWithError(c, &req, apperr.ErrPlatformLoginInvalid) {
		return
	}
	res, err := a.svc.LoginPlatform(c.Request.Context(), req, userAgent(c), clientIP(c))
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, res)
}

// loginPhone 手机号密码登录。
func (a *API) loginPhone(c *gin.Context) {
	var req LoginPhoneRequest
	if !httpx.BindJSONWithError(c, &req, apperr.ErrLoginPhoneInvalid) {
		return
	}
	res, err := a.svc.LoginByPhone(c.Request.Context(), req, userAgent(c), clientIP(c))
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, res)
}

// loginNo 学号/工号登录。
func (a *API) loginNo(c *gin.Context) {
	var req LoginNoRequest
	if !httpx.BindJSONWithError(c, &req, apperr.ErrLoginNoInvalid) {
		return
	}
	res, err := a.svc.LoginByNo(c.Request.Context(), req, userAgent(c), clientIP(c))
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, res)
}

// loginSms 短信验证码登录。
func (a *API) loginSms(c *gin.Context) {
	var req LoginSmsRequest
	if !httpx.BindJSONWithError(c, &req, apperr.ErrLoginSmsInvalid) {
		return
	}
	res, err := a.svc.LoginBySms(c.Request.Context(), req, userAgent(c), clientIP(c))
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, res)
}

// ssoLogin 生成 CAS 登录跳转地址。
func (a *API) ssoLogin(c *gin.Context) {
	serviceURL := c.Query("service")
	if serviceURL == "" {
		response.Fail(c, apperr.ErrSsoLoginInvalid)
		return
	}
	redirectURL, err := a.svc.BuildSsoLoginURL(c.Request.Context(), c.Param("tenant_code"), serviceURL)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, SsoLoginURLResponse{RedirectURL: redirectURL})
}

// ssoCallback 处理 CAS 回调并签发租户账号 Token。
func (a *API) ssoCallback(c *gin.Context) {
	ticket := c.Query("ticket")
	serviceURL := c.Query("service")
	if ticket == "" || serviceURL == "" {
		response.Fail(c, apperr.ErrSsoCallbackInvalid)
		return
	}
	res, err := a.svc.LoginByCasCallback(c.Request.Context(), c.Param("tenant_code"), ticket, serviceURL, userAgent(c), clientIP(c))
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, res)
}

// ssoLDAPLogin 用学校 LDAP 配置校验账号密码并签发租户账号 Token。
func (a *API) ssoLDAPLogin(c *gin.Context) {
	var req LDAPLoginRequest
	if !httpx.BindJSONWithError(c, &req, apperr.ErrLDAPLoginInvalid) {
		return
	}
	res, err := a.svc.LoginByLDAP(c.Request.Context(), c.Param("tenant_code"), req, userAgent(c), clientIP(c))
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, res)
}

// sendSms 发送验证码。
func (a *API) sendSms(c *gin.Context) {
	var req SendSmsRequest
	if !httpx.BindJSONWithError(c, &req, apperr.ErrSmsRequestInvalid) {
		return
	}
	if err := a.svc.SendSms(c.Request.Context(), req); err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, gin.H{"sent": true})
}

// refresh 刷新双 Token。
func (a *API) refresh(c *gin.Context) {
	var req RefreshRequest
	if !httpx.BindJSONWithError(c, &req, apperr.ErrRefreshRequestInvalid) {
		return
	}
	pair, err := a.svc.Refresh(c.Request.Context(), req, userAgent(c), clientIP(c))
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, pair)
}

// resetPassword 找回密码。
func (a *API) resetPassword(c *gin.Context) {
	var req ResetPasswordRequest
	if !httpx.BindJSONWithError(c, &req, apperr.ErrResetPasswordInvalid) {
		return
	}
	if err := a.svc.ResetPassword(c.Request.Context(), req); err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, gin.H{"reset": true})
}

// activateAccount 使用一次性激活码自设密码并激活账号。
func (a *API) activateAccount(c *gin.Context) {
	var req ActivateAccountRequest
	if !httpx.BindJSONWithError(c, &req, apperr.ErrActivationInvalid) {
		return
	}
	if err := a.svc.ActivateAccount(c.Request.Context(), req); err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, gin.H{"activated": true})
}

// logout 登出(吊销当前会话)。
func (a *API) logout(c *gin.Context) {
	id, ok := currentID(c)
	if !ok {
		response.Fail(c, apperr.ErrUnauthorized)
		return
	}
	sessionID, _ := c.Get("session_id")
	sid, _ := sessionID.(int64)
	if err := a.svc.Logout(c.Request.Context(), id.TenantID, id.AccountID, sid); err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, gin.H{"logout": true})
}
