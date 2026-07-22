// notify api 文件负责注册 M10 HTTP 和 WebSocket 路由。
package notify

import (
	"chaimir/internal/contracts"
	"chaimir/internal/platform/auth"
	"chaimir/internal/platform/httpx"
	"chaimir/internal/platform/tenant"
	"chaimir/internal/platform/ws"
	"chaimir/pkg/apperr"

	"github.com/gin-gonic/gin"
)

// RegisterRoutes 注册通知模块 HTTP API。
func RegisterRoutes(r gin.IRouter, svc *Service, authn *auth.Manager, roles contracts.IdentityService) error {
	if r == nil || svc == nil || authn == nil {
		return apperr.ErrHTTPServiceMissing
	}
	api := notifyAPI{svc: svc}
	g := r.Group("/api/v1/notify")
	user := g.Group("", authn.Middleware(), auth.RequireTenantAnyRole(roles, contracts.RoleStudent, contracts.RoleTeacher, contracts.RoleSchoolAdmin))
	announcementReader := g.Group("", authn.Middleware(), auth.RequirePlatformOrAnyRole(roles, contracts.RoleStudent, contracts.RoleTeacher, contracts.RoleSchoolAdmin))
	admin := g.Group("", authn.Middleware(), auth.RequirePlatformOrAnyRole(roles, contracts.RoleSchoolAdmin))
	internal := g.Group("/internal", authn.ServiceMiddleware())
	user.GET("/inbox", api.inbox)
	user.GET("/inbox/unread-count", api.unread)
	user.POST("/inbox/:id/read", api.markRead)
	user.POST("/inbox/read-all", api.markAllRead)
	user.DELETE("/inbox/:id", api.deleteNotification)
	user.GET("/preferences", api.listPreferences)
	user.PUT("/preferences", api.upsertPreference)
	announcementReader.GET("/announcements", api.listAnnouncements)
	user.POST("/announcements/:id/read", api.markAnnouncementRead)
	r.GET("/api/ws", authn.WebSocketMiddleware(), auth.RequireTenantAnyRole(roles, contracts.RoleStudent, contracts.RoleTeacher, contracts.RoleSchoolAdmin), api.websocket)
	admin.POST("/announcements", api.createAnnouncement)
	internal.POST("/send", api.send)
	internal.POST("/push", api.push)
	return nil
}

type notifyAPI struct{ svc *Service }

// inbox 查询当前用户站内信分页。
func (a notifyAPI) inbox(c *gin.Context) {
	page, size, ok := httpx.Page(c)
	if !ok {
		return
	}
	var isRead *bool
	if raw := c.Query("is_read"); raw == "true" || raw == "false" {
		v := raw == "true"
		isRead = &v
	}
	items, total, err := a.svc.Inbox(c.Request.Context(), isRead, c.Query("type"), page, size)
	httpx.WritePage(c, items, total, page, size, err)
}

// unread 查询当前用户未读通知数。
func (a notifyAPI) unread(c *gin.Context) {
	out, err := a.svc.Unread(c.Request.Context())
	httpx.Write(c, out, err)
}

// markRead 标记单条通知已读。
func (a notifyAPI) markRead(c *gin.Context) {
	id, ok := httpx.PathID(c, "id")
	if ok {
		out1, err := a.svc.MarkRead(c.Request.Context(), id)
		httpx.Write(c, out1, err)
	}
}

// markAllRead 标记当前用户全部通知已读。
func (a notifyAPI) markAllRead(c *gin.Context) {
	httpx.Write(c, gin.H{}, a.svc.MarkAllRead(c.Request.Context()))
}

// deleteNotification 删除当前用户的一条通知。
func (a notifyAPI) deleteNotification(c *gin.Context) {
	id, ok := httpx.PathID(c, "id")
	if ok {
		out2, err := a.svc.DeleteNotification(c.Request.Context(), id)
		httpx.Write(c, out2, err)
	}
}

// listPreferences 查询当前用户通知偏好。
func (a notifyAPI) listPreferences(c *gin.Context) {
	out, err := a.svc.ListPreferences(c.Request.Context())
	httpx.Write(c, out, err)
}

// upsertPreference 绑定通知偏好更新请求。
func (a notifyAPI) upsertPreference(c *gin.Context) {
	var req PreferenceRequest
	if !httpx.BindJSONWithError(c, &req, apperr.ErrNotifyRequestInvalid) {
		return
	}
	out3, err := a.svc.UpsertPreference(c.Request.Context(), req)
	httpx.Write(c, out3, err)
}

// createAnnouncement 绑定系统公告创建请求。
func (a notifyAPI) createAnnouncement(c *gin.Context) {
	var req AnnouncementRequest
	if !httpx.BindJSONWithError(c, &req, apperr.ErrNotifyAnnouncementInvalid) {
		return
	}
	out4, err := a.svc.CreateAnnouncement(c.Request.Context(), req)
	httpx.Write(c, out4, err)
}

// listAnnouncements 查询当前用户可见公告。
func (a notifyAPI) listAnnouncements(c *gin.Context) {
	page, size, ok := httpx.Page(c)
	if !ok {
		return
	}
	items, total, p, s, err := a.svc.ListAnnouncements(c.Request.Context(), page, size)
	httpx.WritePage(c, items, total, p, s, err)
}

// markAnnouncementRead 标记公告已读。
func (a notifyAPI) markAnnouncementRead(c *gin.Context) {
	id, ok := httpx.PathID(c, "id")
	if ok {
		httpx.Write(c, gin.H{}, a.svc.MarkAnnouncementRead(c.Request.Context(), id))
	}
}

// send 绑定内部服务通知发送请求。
func (a notifyAPI) send(c *gin.Context) {
	var req SendRequest
	if !httpx.BindJSONWithError(c, &req, apperr.ErrNotifyRequestInvalid) {
		return
	}
	if !serviceTenantMatches(c, req.TenantID.Int64()) {
		return
	}
	httpx.Write(c, gin.H{}, a.svc.Send(c.Request.Context(), contracts.NotifySendRequest{TenantID: req.TenantID.Int64(), Type: req.Type, Receivers: req.Receivers, Params: req.Params, Link: req.Link}))
}

// push 绑定内部服务实时推送请求。
func (a notifyAPI) push(c *gin.Context) {
	var req PushRequest
	if !httpx.BindJSONWithError(c, &req, apperr.ErrNotifySubscribeInvalid) {
		return
	}
	if !serviceTenantMatches(c, req.TenantID.Int64()) {
		return
	}
	httpx.Write(c, gin.H{}, a.svc.Push(c.Request.Context(), contracts.NotifyPushRequest{TenantID: req.TenantID.Int64(), Topic: req.Topic, Payload: req.Payload}))
}

// serviceTenantMatches 确保内部通知请求正文的租户与已验签服务租户一致。
func serviceTenantMatches(c *gin.Context, bodyTenantID int64) bool {
	id, ok := tenant.FromContext(c.Request.Context())
	if !ok || !id.IsSystem || id.TenantID <= 0 || bodyTenantID != id.TenantID {
		httpx.Write(c, gin.H{}, apperr.ErrServiceUnauthorized)
		return false
	}
	return true
}

// websocket 建立通知订阅 WebSocket。
func (a notifyAPI) websocket(c *gin.Context) {
	err := a.svc.hub.ServeInteractive(c.Writer, c.Request, func(conn *ws.Conn) error {
		return a.svc.HandleSubscribe(c.Request.Context(), conn)
	})
	if err != nil {
		httpx.Write(c, gin.H{}, apperr.ErrNotifyChannelUnavailable.WithCause(err))
	}
}
