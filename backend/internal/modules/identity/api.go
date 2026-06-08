// M1 HTTP 接口层(api):路由注册、Handler 结构、鉴权/权限中间件、公共辅助。
// 依据 docs/01 §3 接口、§4 权限矩阵、docs/总-API §1/§2(/api/v1 前缀、kebab 资源)。
package identity

import (
	"context"

	"chaimir/internal/contracts"
	"chaimir/internal/platform/auth"
	"chaimir/internal/platform/config"
	"chaimir/internal/platform/tenant"
	"chaimir/pkg/apperr"
	"chaimir/pkg/response"

	"github.com/gin-gonic/gin"
)

// API 是 M1 的 HTTP 处理器,持有 service 与鉴权依赖。
type API struct {
	svc       *Service
	authMgr   *auth.Manager
	deployCfg config.DeployConfig
	uploadCfg config.UploadConfig
}

// NewAPI 构造带上传边界配置的 API 层。
func NewAPI(svc *Service, authMgr *auth.Manager, deployCfg config.DeployConfig, uploadCfg config.UploadConfig) *API {
	return &API{svc: svc, authMgr: authMgr, deployCfg: deployCfg, uploadCfg: uploadCfg}
}

// Register 在 /api/v1 路由组下注册 M1 全部路由。
// 公开路由(登录/发码/找回/入驻申请)无需鉴权;其余经 authMgr.Middleware() 鉴权,
//
//	并按 platform/auth 统一角色守卫限制;平台层路由受 PLATFORM_LAYER_ENABLED 控制。
func (a *API) Register(rg *gin.RouterGroup) {
	// ---- 公开:认证 ----
	authG := rg.Group("/auth")
	{
		authG.POST("/login/platform", a.loginPlatform)
		authG.POST("/login/phone", a.loginPhone)
		authG.POST("/login/no", a.loginNo)
		authG.POST("/login/sms", a.loginSms)
		authG.GET("/sso/:tenant_code/login", a.ssoLogin)
		authG.GET("/sso/:tenant_code/callback", a.ssoCallback)
		authG.POST("/sso/:tenant_code/ldap", a.ssoLDAPLogin)
		authG.POST("/sms/send", a.sendSms)
		authG.POST("/refresh", a.refresh)
		authG.POST("/password/reset", a.resetPassword)
		authG.POST("/activate", a.activateAccount)
		authG.POST("/logout", a.authMgr.Middleware(), a.logout)
	}

	// ---- 平台管理员:入驻审核 + 租户管理(私有化关闭)----
	if a.deployCfg.PlatformEnabled {
		// 入驻申请提交是公开的(访客)。
		rg.POST("/platform/applications", a.createApplication)
		platG := rg.Group("/platform", a.authMgr.Middleware(), a.requirePlatformAdmin())
		{
			platG.GET("/applications", a.listApplications)
			platG.POST("/applications/:id/approve", a.approveApplication)
			platG.POST("/applications/:id/reject", a.rejectApplication)
			platG.GET("/tenants", a.listTenants)
			platG.GET("/tenants/:id", a.getTenant)
			platG.PATCH("/tenants/:id", a.updateTenant)
		}
	}

	// 以下均需登录。
	authed := rg.Group("", a.authMgr.Middleware())

	// ---- 学校管理员:本校配置 ----
	tenantG := authed.Group("/tenant", auth.RequireTenantAnyRole(a.svc, contracts.RoleSchoolAdmin))
	{
		tenantG.GET("/config", a.getTenantConfig)
		tenantG.PATCH("/config", a.updateTenantConfig)
		tenantG.GET("/sso", a.getSsoConfig)
		tenantG.PUT("/sso", a.upsertSsoConfig)
	}

	// ---- 组织:管理员可写,教师只读 ----
	orgG := authed.Group("/org", a.requireOrgReadAccess())
	{
		orgG.GET("/departments", a.listDepartments)
		orgG.GET("/majors", a.listMajors)
		orgG.GET("/classes", a.listClasses)
		// 写操作仅学校管理员。
		orgW := orgG.Group("", auth.RequireTenantAnyRole(a.svc, contracts.RoleSchoolAdmin))
		{
			orgW.POST("/departments", a.createDepartment)
			orgW.PATCH("/departments/:id", a.updateDepartment)
			orgW.DELETE("/departments/:id", a.deleteDepartment)
			orgW.POST("/majors", a.createMajor)
			orgW.PATCH("/majors/:id", a.updateMajor)
			orgW.DELETE("/majors/:id", a.deleteMajor)
			orgW.POST("/classes", a.createClass)
			orgW.PATCH("/classes/:id", a.updateClass)
			orgW.DELETE("/classes/:id", a.deleteClass)
			orgW.POST("/import", a.importOrg)
			orgW.POST("/classes/archive", a.batchArchiveClasses)
			orgW.POST("/classes/promote", a.batchPromoteClasses)
		}
	}

	// ---- 账号管理:仅学校管理员 ----
	accG := authed.Group("/accounts", auth.RequireTenantAnyRole(a.svc, contracts.RoleSchoolAdmin))
	{
		accG.GET("", a.listAccounts)
		accG.POST("", a.createAccount)
		accG.PATCH("/:id", a.updateAccount)
		accG.POST("/:id/disable", a.disableAccount)
		accG.POST("/:id/enable", a.enableAccount)
		accG.POST("/:id/archive", a.archiveAccount)
		accG.POST("/:id/restore", a.restoreAccount)
		accG.POST("/:id/cancel", a.cancelAccount)
		accG.POST("/:id/force-logout", a.forceLogout)
		accG.POST("/:id/reset-password", a.resetAccountPassword)
		accG.POST("/:id/grant-admin", a.grantAdmin)
		accG.POST("/:id/revoke-admin", a.revokeAdmin)
		accG.GET("/import/template", a.importTemplate)
		accG.GET("/import/batches", a.listImportBatches)
		accG.POST("/import/preview", a.importPreview)
		accG.POST("/import/commit", a.importCommit)
		accG.POST("/batch/disable", a.batchDisableAccounts)
		accG.POST("/batch/archive", a.batchArchiveAccounts)
		accG.POST("/batch/restore", a.batchRestoreAccounts)
	}

	// ---- 个人中心:全角色 ----
	meG := authed.Group("/me")
	{
		meG.GET("", a.getMe)
		meG.POST("/password", a.changeMyPassword)
		meG.POST("/phone", a.changeMyPhone)
		meG.GET("/sessions", a.listMySessions)
	}

	// ---- 审计查询:学校管理员/平台管理员 ----
	authed.GET("/audit", a.requireAuditAccess(), a.listAudit)
}

// ---- 鉴权/权限中间件 ----

// requirePlatformAdmin 要求平台管理员上下文(JWT plat 标记)。
func (a *API) requirePlatformAdmin() gin.HandlerFunc {
	return func(c *gin.Context) {
		id, ok := tenant.FromContext(c.Request.Context())
		if !ok || !id.IsPlatform {
			response.Fail(c, apperr.ErrForbidden)
			c.Abort()
			return
		}
		c.Next()
	}
}

// requireAuditAccess 允许平台管理员查平台审计、学校管理员查本校审计。
func (a *API) requireAuditAccess() gin.HandlerFunc {
	return func(c *gin.Context) {
		if err := authorizeAuditAccess(c.Request.Context(), a.svc.HasRole); err != nil {
			response.Fail(c, err)
			c.Abort()
			return
		}
		c.Next()
	}
}

// requireOrgReadAccess 允许学校管理员和教师查看组织结构,学生不开放该视图。
func (a *API) requireOrgReadAccess() gin.HandlerFunc {
	return func(c *gin.Context) {
		if err := authorizeOrgReadAccess(c.Request.Context(), a.svc.HasRole); err != nil {
			response.Fail(c, err)
			c.Abort()
			return
		}
		c.Next()
	}
}

// roleChecker 是权限 helper 依赖的角色查询函数。
type roleChecker func(ctx context.Context, accountID int64, role string) (bool, error)

// authorizeAuditAccess 判断当前身份是否可查询审计日志。
func authorizeAuditAccess(ctx context.Context, hasRole roleChecker) error {
	id, ok := tenant.FromContext(ctx)
	if !ok {
		return apperr.ErrUnauthorized
	}
	if id.IsPlatform {
		return nil
	}
	has, err := hasRole(ctx, id.AccountID, contracts.RoleCode(RoleSchoolAdmin))
	if err != nil {
		return err
	}
	if !has {
		return apperr.ErrForbidden
	}
	return nil
}

// authorizeOrgReadAccess 判断当前账号是否可查看本校组织结构。
func authorizeOrgReadAccess(ctx context.Context, hasRole roleChecker) error {
	id, ok := tenant.FromContext(ctx)
	if !ok {
		return apperr.ErrUnauthorized
	}

	// 组织只读视图按权限矩阵仅面向学校管理员和教师;逐个查服务端角色,不信任客户端传参。
	for _, role := range []int16{RoleSchoolAdmin, RoleTeacher} {
		has, err := hasRole(ctx, id.AccountID, contracts.RoleCode(role))
		if err != nil {
			return err
		}
		if has {
			return nil
		}
	}
	return apperr.ErrForbidden
}

// ---- 公共辅助 ----

// currentID 取当前鉴权身份。
func currentID(c *gin.Context) (tenant.Identity, bool) {
	return tenant.FromContext(c.Request.Context())
}

// clientIP 取可信代理链解析后的客户端地址,用于审计和会话记录。
func clientIP(c *gin.Context) string { return c.ClientIP() }

// userAgent 取客户端 UA 字符串,仅用于会话展示与审计上下文。
func userAgent(c *gin.Context) string { return c.Request.UserAgent() }
