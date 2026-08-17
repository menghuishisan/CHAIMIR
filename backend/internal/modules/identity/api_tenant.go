// identity api_tenant 文件承接租户配置和统一认证配置 HTTP 请求。
package identity

import (
	"time"

	"chaimir/internal/contracts"
	"chaimir/internal/platform/auth"
	"chaimir/internal/platform/httpx"
	"chaimir/internal/platform/upload"
	"chaimir/pkg/apperr"
	"chaimir/pkg/logging"

	"github.com/gin-gonic/gin"
)

// tenantAPI 封装租户配置 HTTP handler 依赖。
type tenantAPI struct {
	svc *Service
}

// registerTenantRoutes 注册学校管理员维护租户配置和 SSO 配置的路由,以及登录页读取学校品牌的免鉴权入口。
func registerTenantRoutes(r gin.IRouter, svc *Service, authn *auth.Manager) {
	api := tenantAPI{svc: svc}
	// 品牌读取先注册在同一 /tenant 前缀下但不挂鉴权:登录页还没有会话。
	// 它不接受任何租户标识参数,只在私有化部署下有内容,故不存在租户枚举面
	// (见 docs/01-身份与租户/03-接口设计.md)。免鉴权且要读一次对象存储,
	// 所以复用认证入口同一套来源限流,不新开配置项。
	brand := r.Group("/tenant", httpx.RateLimitMiddleware(svc.redis, "identity:brand-rate", svc.cfg.AuthRateMax, time.Duration(svc.cfg.AuthRateWindowSeconds)*time.Second))
	brand.GET("/brand", api.getBrand)
	g := r.Group("/tenant", authn.Middleware(), auth.RequireTenantAnyRole(svc, contracts.RoleSchoolAdmin))
	g.GET("/config", api.getConfig)
	g.PATCH("/config", api.updateConfig)
	g.POST("/logo", api.uploadLogo)
	g.DELETE("/logo", api.clearLogo)
	g.GET("/sso", api.listSSO)
	g.PUT("/sso", api.upsertSSO)
}

// getBrand 读取登录页需要的学校名称与校徽,SaaS 部署返回空品牌。
func (a tenantAPI) getBrand(c *gin.Context) {
	out, err := a.svc.GetTenantBrand(c.Request.Context())
	httpx.Write(c, out, err)
}

// uploadLogo 绑定校徽 multipart 上传;校验、落对象与写入租户配置都在 service 内完成。
// 上传即生效,响应就是更新后的租户配置视图,前端不需要再提交一次引用。
func (a tenantAPI) uploadLogo(c *gin.Context) {
	maxBytes := a.svc.uploadCfg.TenantLogoMaxBytes
	file, err := c.FormFile("file")
	if err != nil {
		httpx.Write(c, gin.H{}, apperr.ErrIdentityTenantLogoInvalid.WithCause(err))
		return
	}
	if file.Size <= 0 {
		httpx.Write(c, gin.H{}, apperr.ErrIdentityTenantLogoInvalid)
		return
	}
	if file.Size > maxBytes {
		httpx.Write(c, gin.H{}, apperr.ErrIdentityTenantLogoTooLarge)
		return
	}
	opened, err := file.Open()
	if err != nil {
		httpx.Write(c, gin.H{}, apperr.ErrIdentityTenantLogoInvalid.WithCause(err))
		return
	}
	defer logging.CloseContext(c.Request.Context(), "关闭校徽上传文件失败", opened)
	content, result, err := upload.ReadBounded(opened, maxBytes)
	if err != nil {
		httpx.Write(c, gin.H{}, apperr.ErrIdentityTenantLogoInvalid.WithCause(err))
		return
	}
	if result != upload.SizeOK {
		httpx.Write(c, gin.H{}, apperr.ErrIdentityTenantLogoTooLarge)
		return
	}
	out, err := a.svc.UploadTenantLogo(c.Request.Context(), TenantLogoUploadRequest{
		FileName:    file.Filename,
		ContentType: file.Header.Get("Content-Type"),
		Content:     content,
	})
	httpx.Write(c, out, err)
}

// clearLogo 移除校徽,徽记位回落学校名首字。
func (a tenantAPI) clearLogo(c *gin.Context) {
	out, err := a.svc.ClearTenantLogo(c.Request.Context())
	if err != nil {
		httpx.Write(c, gin.H{}, err)
		return
	}
	httpx.Write(c, out, nil)
}

// getConfig 读取当前租户配置,API 层不直接访问数据库。
func (a tenantAPI) getConfig(c *gin.Context) {
	out, err := a.svc.GetTenantConfig(c.Request.Context())
	if err != nil {
		httpx.Write(c, gin.H{}, err)
		return
	}
	httpx.Write(c, out, nil)
}

// updateConfig 绑定租户配置更新请求并委托 service 校验和落库。
func (a tenantAPI) updateConfig(c *gin.Context) {
	var req TenantConfigRequest
	if !httpx.BindJSON(c, &req) {
		return
	}
	out, err := a.svc.UpdateTenantConfigByAdmin(c.Request.Context(), req)
	if err != nil {
		httpx.Write(c, gin.H{}, err)
		return
	}
	httpx.Write(c, out, nil)
}

// listSSO 读取当前租户统一认证配置列表,响应前由 service/convert 脱敏。
func (a tenantAPI) listSSO(c *gin.Context) {
	out, err := a.svc.ListSSOConfigsByAdmin(c.Request.Context())
	if err != nil {
		httpx.Write(c, gin.H{}, err)
		return
	}
	httpx.Write(c, out, nil)
}

// upsertSSO 绑定 CAS/LDAP 配置更新请求,敏感字段加密由 service 执行。
func (a tenantAPI) upsertSSO(c *gin.Context) {
	var req SSOConfigRequest
	if !httpx.BindJSON(c, &req) {
		return
	}
	out, err := a.svc.UpsertSSOConfig(c.Request.Context(), req)
	if err != nil {
		httpx.Write(c, gin.H{}, err)
		return
	}
	httpx.Write(c, out, nil)
}
