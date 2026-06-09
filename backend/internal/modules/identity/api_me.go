// identity api_me 文件承接个人中心 HTTP 请求,只做绑定、鉴权组合和统一响应。
package identity

import (
	"chaimir/internal/platform/auth"
	"chaimir/internal/platform/httpx"
	"chaimir/pkg/response"

	"github.com/gin-gonic/gin"
)

// meAPI 封装个人中心 HTTP handler 依赖。
type meAPI struct {
	svc *Service
}

// registerMeRoutes 注册个人中心路由。
func registerMeRoutes(r gin.IRouter, svc *Service, authn *auth.Manager) {
	api := meAPI{svc: svc}
	g := r.Group("/me", authn.Middleware())
	api.register(g)
}

// register 绑定个人中心资源路由到具名 handler。
func (a meAPI) register(g gin.IRouter) {
	g.GET("", a.getMe)
	g.POST("/password", a.changePassword)
	g.POST("/phone", a.changePhone)
	g.GET("/sessions", a.sessions)
}

// getMe 返回当前登录账号个人信息。
func (a meAPI) getMe(c *gin.Context) {
	out, err := a.svc.GetMe(c.Request.Context())
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, out)
}

// changePassword 绑定改密请求并委托 service 校验旧密码。
func (a meAPI) changePassword(c *gin.Context) {
	var req ChangePasswordRequest
	if !httpx.BindJSON(c, &req) {
		return
	}
	if err := a.svc.ChangeMyPassword(c.Request.Context(), req); err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, gin.H{})
}

// changePhone 绑定换绑手机号请求并委托 service 校验短信验证码。
func (a meAPI) changePhone(c *gin.Context) {
	var req ChangePhoneRequest
	if !httpx.BindJSON(c, &req) {
		return
	}
	if err := a.svc.ChangeMyPhone(c.Request.Context(), req); err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, gin.H{})
}

// sessions 返回当前账号服务端会话列表。
func (a meAPI) sessions(c *gin.Context) {
	out, err := a.svc.ListMySessions(c.Request.Context())
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, out)
}
