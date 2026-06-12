// notify api 文件负责注册 M10 HTTP 和 WebSocket 路由。
package notify

import (
	"chaimir/internal/contracts"
	"chaimir/internal/platform/auth"
	"chaimir/internal/platform/httpx"
	"chaimir/internal/platform/ws"
	"chaimir/pkg/apperr"

	"github.com/gin-gonic/gin"
)

// RegisterRoutes 注册通知模块 HTTP API。
func RegisterRoutes(r gin.IRouter, svc *Service, authn *auth.Manager, roles auth.RoleChecker) error {
	if r == nil || svc == nil || authn == nil {
		return apperr.ErrInternal.WithMessage("notify routes 依赖不完整")
	}
	api := notifyAPI{svc: svc}
	g := r.Group("/api/v1/notify")
	user := g.Group("", authn.Middleware(), auth.RequireTenantAnyRole(roles, contracts.RoleStudent, contracts.RoleTeacher, contracts.RoleSchoolAdmin))
	admin := g.Group("", authn.Middleware(), auth.RequireTenantAnyRole(roles, contracts.RoleTeacher, contracts.RoleSchoolAdmin))
	internal := g.Group("/internal", authn.ServiceMiddleware())
	user.GET("/inbox", api.inbox)
	user.GET("/unread", api.unread)
	user.POST("/inbox/:id/read", api.markRead)
	user.POST("/inbox/read-all", api.markAllRead)
	user.DELETE("/inbox/:id", api.deleteNotification)
	user.GET("/preferences", api.listPreferences)
	user.PUT("/preferences", api.upsertPreference)
	user.GET("/announcements", api.listAnnouncements)
	user.POST("/announcements/:id/read", api.markAnnouncementRead)
	user.GET("/ws", api.websocket)
	admin.POST("/announcements", api.createAnnouncement)
	internal.POST("/send", api.send)
	internal.POST("/push", api.push)
	return nil
}

type notifyAPI struct{ svc *Service }

func (a notifyAPI) inbox(c *gin.Context) {
	page, size, ok := notifyPage(c)
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

func (a notifyAPI) unread(c *gin.Context) {
	out, err := a.svc.Unread(c.Request.Context())
	httpx.Write(c, out, err)
}

func (a notifyAPI) markRead(c *gin.Context) {
	id, ok := httpx.PathID(c, "id")
	if ok {
		out1, err := a.svc.MarkRead(c.Request.Context(), id)
		httpx.Write(c, out1, err)
	}
}

func (a notifyAPI) markAllRead(c *gin.Context) {
	httpx.Write(c, gin.H{}, a.svc.MarkAllRead(c.Request.Context()))
}

func (a notifyAPI) deleteNotification(c *gin.Context) {
	id, ok := httpx.PathID(c, "id")
	if ok {
		out2, err := a.svc.DeleteNotification(c.Request.Context(), id)
		httpx.Write(c, out2, err)
	}
}

func (a notifyAPI) listPreferences(c *gin.Context) {
	out, err := a.svc.ListPreferences(c.Request.Context())
	httpx.Write(c, out, err)
}

func (a notifyAPI) upsertPreference(c *gin.Context) {
	var req PreferenceRequest
	if !httpx.BindJSONWithError(c, &req, apperr.ErrNotifyRequestInvalid) {
		return
	}
	out3, err := a.svc.UpsertPreference(c.Request.Context(), req)
	httpx.Write(c, out3, err)
}

func (a notifyAPI) createAnnouncement(c *gin.Context) {
	var req AnnouncementRequest
	if !httpx.BindJSONWithError(c, &req, apperr.ErrNotifyAnnouncementInvalid) {
		return
	}
	out4, err := a.svc.CreateAnnouncement(c.Request.Context(), req)
	httpx.Write(c, out4, err)
}

func (a notifyAPI) listAnnouncements(c *gin.Context) {
	page, size, ok := notifyPage(c)
	if ok {
		out5, err := a.svc.ListAnnouncements(c.Request.Context(), page, size)
		httpx.Write(c, out5, err)
	}
}

func (a notifyAPI) markAnnouncementRead(c *gin.Context) {
	id, ok := httpx.PathID(c, "id")
	if ok {
		httpx.Write(c, gin.H{}, a.svc.MarkAnnouncementRead(c.Request.Context(), id))
	}
}

func (a notifyAPI) send(c *gin.Context) {
	var req SendRequest
	if !httpx.BindJSONWithError(c, &req, apperr.ErrNotifyRequestInvalid) {
		return
	}
	httpx.Write(c, gin.H{}, a.svc.Send(c.Request.Context(), contracts.NotifySendRequest{TenantID: req.TenantID, Type: req.Type, Receivers: req.Receivers, Params: req.Params, Link: req.Link}))
}

func (a notifyAPI) push(c *gin.Context) {
	var req PushRequest
	if !httpx.BindJSONWithError(c, &req, apperr.ErrNotifySubscribeInvalid) {
		return
	}
	httpx.Write(c, gin.H{}, a.svc.Push(c.Request.Context(), contracts.NotifyPushRequest{TenantID: req.TenantID, Topic: req.Topic, Payload: req.Payload}))
}

func (a notifyAPI) websocket(c *gin.Context) {
	err := a.svc.hub.ServeInteractive(c.Writer, c.Request, func(conn *ws.Conn) error {
		return a.svc.HandleSubscribe(c.Request.Context(), conn)
	})
	if err != nil {
		httpx.Write(c, gin.H{}, apperr.ErrNotifyChannelUnavailable.WithCause(err))
	}
}

func notifyPage(c *gin.Context) (int, int, bool) {
	p, ok := httpx.QueryInt(c, "page", httpx.QueryIntRule{Default: 1, Min: 1})
	if !ok {
		return 0, 0, false
	}
	s, ok := httpx.QueryInt(c, "size", httpx.QueryIntRule{Default: 20, Min: 1, Max: 100, HasMax: true})
	if !ok {
		return 0, 0, false
	}
	return int(p), int(s), true
}
