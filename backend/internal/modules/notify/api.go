// M10 HTTP 接口层:注册通知发送、实时推送、站内信、偏好和系统公告路由。
package notify

import (
	"context"
	"strconv"

	"chaimir/internal/contracts"
	"chaimir/internal/platform/auth"
	"chaimir/internal/platform/httpx"
	"chaimir/internal/platform/pagex"
	"chaimir/internal/platform/tenant"
	"chaimir/internal/platform/ws"
	"chaimir/pkg/apperr"
	"chaimir/pkg/response"

	"github.com/gin-gonic/gin"
)

// notifyService 是 M10 API 依赖的服务接口,便于路由鉴权测试注入替身。
type notifyService interface {
	Send(context.Context, contracts.NotifySendRequest) error
	Push(context.Context, contracts.NotifyPushRequest) error
	ListInbox(context.Context, InboxQuery) ([]NotificationDTO, int64, error)
	UnreadCount(context.Context) (int64, error)
	MarkNotificationRead(context.Context, int64) error
	MarkAllNotificationsRead(context.Context) error
	DeleteNotification(context.Context, int64) error
	ListPreferences(context.Context) ([]PreferenceDTO, error)
	UpdatePreferences(context.Context, []PreferenceRequest) error
	CreateAnnouncement(context.Context, AnnouncementRequest) (AnnouncementDTO, error)
	ListAnnouncements(context.Context, []int16) ([]AnnouncementDTO, error)
	MarkAnnouncementRead(context.Context, int64) error
}

// API 是 M10 的 HTTP 处理器。
type API struct {
	svc      notifyService
	authMgr  *auth.Manager
	identity contracts.IdentityService
	hub      *ws.Hub
}

// NewAPI 构造 M10 API。
func NewAPI(svc notifyService, authMgr *auth.Manager, identity contracts.IdentityService, hub *ws.Hub) *API {
	return &API{svc: svc, authMgr: authMgr, identity: identity, hub: hub}
}

// Register 注册 M10 路由:内部发送/推送走服务鉴权,用户站内信与公告走登录鉴权。
func (a *API) Register(rg *gin.RouterGroup) {
	internal := rg.Group("/notify", a.authMgr.ServiceMiddleware())
	{
		internal.POST("/send", a.send)
		internal.POST("/push", a.push)
	}
	g := rg.Group("/notify", a.authMgr.Middleware())
	{
		g.GET("/inbox", a.listInbox)
		g.GET("/inbox/unread-count", a.unreadCount)
		g.POST("/inbox/:id/read", a.markRead)
		g.POST("/inbox/read-all", a.markAllRead)
		g.DELETE("/inbox/:id", a.deleteNotification)
		g.GET("/preferences", a.listPreferences)
		g.PUT("/preferences", a.updatePreferences)
		g.POST("/announcements", a.requireAnnouncementPublisher(), a.createAnnouncement)
		g.GET("/announcements", a.listAnnouncements)
		g.POST("/announcements/:id/read", a.markAnnouncementRead)
	}
	rg.GET("/ws", a.ws)
}

// requireAnnouncementPublisher 要求平台管理员或学校管理员发布系统公告。
func (a *API) requireAnnouncementPublisher() gin.HandlerFunc {
	return auth.RequirePlatformOrAnyRole(a.identity, contracts.RoleSchoolAdmin)
}

// send 处理内部统一通知发送请求。
func (a *API) send(c *gin.Context) {
	var req contracts.NotifySendRequest
	if httpx.BindJSONWithError(c, &req, apperr.ErrNotifyInvalid) {
		httpx.Write(c, map[string]any{"sent": true}, a.svc.Send(c.Request.Context(), req))
	}
}

// push 处理内部实时 topic 推送请求。
func (a *API) push(c *gin.Context) {
	var req contracts.NotifyPushRequest
	if httpx.BindJSONWithError(c, &req, apperr.ErrNotifyInvalid) {
		httpx.Write(c, map[string]any{"pushed": true}, a.svc.Push(c.Request.Context(), req))
	}
}

// listInbox 查询当前用户收件箱。
func (a *API) listInbox(c *gin.Context) {
	query := InboxQuery{Type: c.Query("type"), Page: httpx.Int(c.Query("page")), Size: httpx.Int(c.Query("size"))}
	if c.Query("is_read") != "" {
		v, err := strconv.ParseBool(c.Query("is_read"))
		if err != nil {
			response.Fail(c, apperr.ErrNotifyInboxQueryInvalid)
			return
		}
		query.IsRead = &v
	}
	items, total, err := a.svc.ListInbox(c.Request.Context(), query)
	page, size := pagex.Normalize(query.Page, query.Size)
	httpx.WritePage(c, items, total, page, size, err)
}

// unreadCount 返回当前用户未读站内信数量。
func (a *API) unreadCount(c *gin.Context) {
	count, err := a.svc.UnreadCount(c.Request.Context())
	httpx.Write(c, map[string]any{"unread_count": count}, err)
}

// markRead 标记一条站内信已读。
func (a *API) markRead(c *gin.Context) {
	id, ok := httpx.PathID(c, "id")
	if ok {
		httpx.Write(c, map[string]any{"read": true}, a.svc.MarkNotificationRead(c.Request.Context(), id))
	}
}

// markAllRead 标记当前用户全部站内信已读。
func (a *API) markAllRead(c *gin.Context) {
	httpx.Write(c, map[string]any{"read": true}, a.svc.MarkAllNotificationsRead(c.Request.Context()))
}

// deleteNotification 软删一条站内信。
func (a *API) deleteNotification(c *gin.Context) {
	id, ok := httpx.PathID(c, "id")
	if ok {
		httpx.Write(c, map[string]any{"deleted": true}, a.svc.DeleteNotification(c.Request.Context(), id))
	}
}

// listPreferences 查询当前用户通知偏好。
func (a *API) listPreferences(c *gin.Context) {
	out, err := a.svc.ListPreferences(c.Request.Context())
	httpx.Write(c, out, err)
}

// updatePreferences 更新当前用户通知偏好。
func (a *API) updatePreferences(c *gin.Context) {
	var req []PreferenceRequest
	if httpx.BindJSONWithError(c, &req, apperr.ErrNotifyInvalid) {
		httpx.Write(c, map[string]any{"updated": true}, a.svc.UpdatePreferences(c.Request.Context(), req))
	}
}

// createAnnouncement 发布一条系统公告。
func (a *API) createAnnouncement(c *gin.Context) {
	var req AnnouncementRequest
	if httpx.BindJSONWithError(c, &req, apperr.ErrNotifyAnnouncementInvalid) {
		if err := a.validateAnnouncementScope(c, req.Scope); err != nil {
			response.Fail(c, err)
			return
		}
		out, err := a.svc.CreateAnnouncement(c.Request.Context(), req)
		httpx.Write(c, out, err)
	}
}

// listAnnouncements 查询当前用户可见公告。
func (a *API) listAnnouncements(c *gin.Context) {
	roles, err := a.currentRoleNums(c)
	if err != nil {
		response.Fail(c, err)
		return
	}
	out, err := a.svc.ListAnnouncements(c.Request.Context(), roles)
	httpx.Write(c, out, err)
}

// markAnnouncementRead 标记一条公告已读。
func (a *API) markAnnouncementRead(c *gin.Context) {
	id, ok := httpx.PathID(c, "id")
	if ok {
		httpx.Write(c, map[string]any{"read": true}, a.svc.MarkAnnouncementRead(c.Request.Context(), id))
	}
}

// ws 建立 M10 统一实时通道,token 通过查询参数传入以适配浏览器 WebSocket API。
func (a *API) ws(c *gin.Context) {
	if err := ServeWS(a.hub, a.authMgr, c.Writer, c.Request); err != nil {
		response.Fail(c, err)
	}
}

// validateAnnouncementScope 校验公告发布范围与管理员身份匹配。
func (a *API) validateAnnouncementScope(c *gin.Context, scope int16) error {
	id, ok := tenant.FromContext(c.Request.Context())
	if !ok {
		return apperr.ErrUnauthorized
	}
	if scope != AnnouncementScopePlatform && scope != AnnouncementScopeTenant && scope != AnnouncementScopeRole {
		return apperr.ErrNotifyAnnouncementInvalid
	}
	if id.IsPlatform {
		if scope != AnnouncementScopePlatform {
			return apperr.ErrNotifyAnnouncementInvalid
		}
		return nil
	}
	if id.TenantID <= 0 || scope == AnnouncementScopePlatform {
		return apperr.ErrForbidden
	}
	return nil
}

// currentRoleNums 读取当前账号角色并转换为公告 target_roles 使用的枚举值。
func (a *API) currentRoleNums(c *gin.Context) ([]int16, error) {
	id, ok := tenant.FromContext(c.Request.Context())
	if !ok {
		return nil, apperr.ErrUnauthorized
	}
	if id.IsPlatform {
		return nil, nil
	}
	if a.identity == nil {
		return nil, apperr.ErrForbidden
	}
	account, err := a.identity.GetAccount(c.Request.Context(), id.AccountID)
	if err != nil {
		return nil, apperr.ErrForbidden.WithCause(err)
	}
	out := make([]int16, 0, len(account.Roles))
	for _, role := range account.Roles {
		if n, ok := contracts.RoleNumber(role); ok {
			out = append(out, n)
		}
	}
	return out, nil
}
