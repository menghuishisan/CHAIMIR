// M10 服务层:实现模板渲染、偏好过滤、站内信、公告与实时推送业务规则。
package notify

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"chaimir/internal/contracts"
	"chaimir/internal/platform/config"
	"chaimir/internal/platform/db"
	"chaimir/internal/platform/redis"
	"chaimir/internal/platform/tenant"
	"chaimir/internal/platform/ws"
	"chaimir/pkg/apperr"
	"chaimir/pkg/logging"
	"chaimir/pkg/snowflake"
)

// notifyStore 抽象 M10 数据访问,便于服务规则测试。
type notifyStore interface {
	GetTemplate(context.Context, string) (TemplateDTO, error)
	GetPreference(context.Context, int64, int64, string) (bool, bool, error)
	CreateNotifications(context.Context, int64, []NotificationCreate) error
	ListInbox(context.Context, int64, int64, InboxQuery) ([]NotificationDTO, int64, error)
	CountUnreadNotifications(context.Context, int64, int64) (int64, error)
	MarkNotificationRead(context.Context, int64, int64) error
	MarkAllNotificationsRead(context.Context, int64) error
	SoftDeleteNotification(context.Context, int64, int64) error
	ListPreferences(context.Context, int64, int64) ([]PreferenceDTO, error)
	UpsertPreferences(context.Context, int64, int64, []PreferenceRequest) error
	CreateAnnouncement(context.Context, int64, int64, AnnouncementRequest) (AnnouncementDTO, error)
	ListAnnouncements(context.Context, int64, int64, []int16) ([]AnnouncementDTO, error)
	GetAnnouncement(context.Context, int64, int64) (AnnouncementDTO, error)
	MarkAnnouncementRead(context.Context, int64, int64, int64, int64) error
}

// unreadCounter 抽象 Redis 未读计数,服务层只关心计数语义。
type unreadCounter interface {
	Increment(context.Context, int64, int64) (int64, error)
	Get(context.Context, int64, int64) (int64, bool, error)
	Set(context.Context, int64, int64, int64) error
	Reset(context.Context, int64, int64) error
}

// sendRateLimiter 抽象通知发送限频,用于拦截事件风暴和异常内部调用。
type sendRateLimiter interface {
	Allow(context.Context, int64, string) (bool, error)
}

// realtimeBroadcaster 抽象 WebSocket 广播能力,便于测试断言 topic 与载荷。
type realtimeBroadcaster interface {
	Broadcast(topic string, payload map[string]any) error
}

// Service 是 M10 通知与实时推送服务。
type Service struct {
	store             notifyStore
	idgen             snowflake.Generator
	unread            unreadCounter
	rateLimiter       sendRateLimiter
	broadcaster       realtimeBroadcaster
	eventRetryMax     int
	eventRetryDelayMs int
	waitRetryDelay    func(context.Context, int) error
}

// NewService 构造 M10 服务并注入数据库、Redis 与 WebSocket Hub。
func NewService(database *db.DB, idgen *snowflake.Node, redisClient *redis.Client, hub *ws.Hub, cfg config.NotifyConfig) *Service {
	return &Service{
		store:             newRepo(database, idgen),
		idgen:             idgen,
		unread:            newRedisUnreadCounter(redisClient, time.Duration(cfg.UnreadTTLHours)*time.Hour),
		rateLimiter:       newRedisSendRateLimiter(redisClient, time.Duration(cfg.SendRateWindowSeconds)*time.Second, cfg.SendRateMax),
		broadcaster:       newHubBroadcaster(hub),
		eventRetryMax:     normalizeRetryMax(cfg.EventRetryMax),
		eventRetryDelayMs: cfg.EventRetryDelayMs,
		waitRetryDelay:    waitNotifyRetryDelay,
	}
}

// Send 渲染模板并发送站内信,再向在线客户端推送个人未读红点。
func (s *Service) Send(ctx context.Context, req contracts.NotifySendRequest) error {
	// 第一步先做请求边界和发送限频,防止事件风暴放大成批量站内信写入。
	if err := validateSendRequest(req); err != nil {
		return err
	}
	if err := s.enforceSendRate(ctx, req.TenantID, req.Type); err != nil {
		return err
	}
	tpl, err := s.store.GetTemplate(ctx, req.Type)
	if err != nil {
		return err
	}
	rows := make([]NotificationCreate, 0, len(req.Receivers))
	// 第二步按接收人偏好过滤并渲染模板,强制模板仍可绕过用户关闭项。
	for _, receiverID := range req.Receivers {
		enabled, found, err := s.store.GetPreference(ctx, req.TenantID, receiverID, req.Type)
		if err != nil {
			return apperr.ErrNotifySendFailed.WithCause(err)
		}
		if found && !enabled && !tpl.Force {
			continue
		}
		row := NotificationCreate{
			ID:         s.nextID(),
			TenantID:   req.TenantID,
			ReceiverID: receiverID,
			Type:       req.Type,
			Title:      renderTemplate(tpl.TitleTemplate, req.Params),
			Content:    renderTemplate(tpl.ContentTemplate, req.Params),
			Link:       strings.TrimSpace(req.Link),
		}
		rows = append(rows, row)
	}
	if len(rows) == 0 {
		return nil
	}
	// 第三步批量落库后再推送红点,Redis/WS 失败只影响实时性不影响权威通知记录。
	if err := s.store.CreateNotifications(ctx, req.TenantID, rows); err != nil {
		return apperr.ErrNotifySendFailed.WithCause(err)
	}
	for _, row := range rows {
		if err := s.pushUnread(ctx, req.TenantID, row.ReceiverID); err != nil {
			logging.ErrorContext(ctx, "通知红点推送失败", err.Error(),
				slog.Int64("tenant_id", req.TenantID),
				slog.Int64("receiver_id", row.ReceiverID),
				slog.String("type", req.Type),
			)
		}
	}
	return nil
}

// enforceSendRate 在渲染和写库前执行发送限频,避免事件风暴放大为批量站内信。
func (s *Service) enforceSendRate(ctx context.Context, tenantID int64, typ string) error {
	if s.rateLimiter == nil {
		return apperr.ErrNotifySendFailed
	}
	allowed, err := s.rateLimiter.Allow(ctx, tenantID, typ)
	if err != nil {
		return apperr.ErrNotifySendFailed.WithCause(err)
	}
	if !allowed {
		return apperr.ErrNotifyRateLimited
	}
	return nil
}

// Push 向订阅业务 topic 的在线连接推送实时载荷。
func (s *Service) Push(ctx context.Context, req contracts.NotifyPushRequest) error {
	if req.TenantID <= 0 || strings.TrimSpace(req.Topic) == "" {
		return apperr.ErrNotifyInvalid
	}
	if s.broadcaster == nil {
		return apperr.ErrNotifyPushFailed
	}
	envelope := map[string]any{
		"topic":   req.Topic,
		"payload": req.Payload,
	}
	if err := s.broadcaster.Broadcast(tenantTopic(req.TenantID, req.Topic), envelope); err != nil {
		return apperr.ErrNotifyPushFailed.WithCause(err)
	}
	return nil
}

// ListInbox 查询当前用户站内信。
func (s *Service) ListInbox(ctx context.Context, query InboxQuery) ([]NotificationDTO, int64, error) {
	id, err := currentTenant(ctx)
	if err != nil {
		return nil, 0, err
	}
	return s.store.ListInbox(ctx, id.TenantID, id.AccountID, query)
}

// UnreadCount 返回当前用户未读站内信数量。
func (s *Service) UnreadCount(ctx context.Context) (int64, error) {
	id, err := currentTenant(ctx)
	if err != nil {
		return 0, err
	}
	if s.unread == nil {
		return 0, apperr.ErrNotifySendFailed
	}
	count, cached, err := s.unread.Get(ctx, id.TenantID, id.AccountID)
	if err != nil {
		return 0, apperr.ErrNotifySendFailed.WithCause(err)
	}
	if cached {
		return count, nil
	}
	count, err = s.store.CountUnreadNotifications(ctx, id.TenantID, id.AccountID)
	if err != nil {
		return 0, apperr.ErrNotifySendFailed.WithCause(err)
	}
	if err := s.unread.Set(ctx, id.TenantID, id.AccountID, count); err != nil {
		return 0, apperr.ErrNotifySendFailed.WithCause(err)
	}
	return count, nil
}

// MarkNotificationRead 标记当前用户的一条站内信已读。
func (s *Service) MarkNotificationRead(ctx context.Context, notificationID int64) error {
	id, err := currentTenant(ctx)
	if err != nil {
		return err
	}
	if notificationID <= 0 {
		return apperr.ErrNotifyNotFound
	}
	if err := s.store.MarkNotificationRead(ctx, id.AccountID, notificationID); err != nil {
		return err
	}
	s.refreshUnreadDot(ctx, id.TenantID, id.AccountID)
	return nil
}

// MarkAllNotificationsRead 标记当前用户全部站内信已读并清空未读计数。
func (s *Service) MarkAllNotificationsRead(ctx context.Context) error {
	id, err := currentTenant(ctx)
	if err != nil {
		return err
	}
	if err := s.store.MarkAllNotificationsRead(ctx, id.AccountID); err != nil {
		return err
	}
	s.refreshUnreadDot(ctx, id.TenantID, id.AccountID)
	return nil
}

// DeleteNotification 对当前用户的一条站内信执行软删。
func (s *Service) DeleteNotification(ctx context.Context, notificationID int64) error {
	id, err := currentTenant(ctx)
	if err != nil {
		return err
	}
	if notificationID <= 0 {
		return apperr.ErrNotifyNotFound
	}
	if err := s.store.SoftDeleteNotification(ctx, id.AccountID, notificationID); err != nil {
		return err
	}
	s.refreshUnreadDot(ctx, id.TenantID, id.AccountID)
	return nil
}

// ListPreferences 查询当前用户通知偏好。
func (s *Service) ListPreferences(ctx context.Context) ([]PreferenceDTO, error) {
	id, err := currentTenant(ctx)
	if err != nil {
		return nil, err
	}
	return s.store.ListPreferences(ctx, id.TenantID, id.AccountID)
}

// UpdatePreferences 更新当前用户通知偏好,强制模板不能关闭。
func (s *Service) UpdatePreferences(ctx context.Context, preferences []PreferenceRequest) error {
	id, err := currentTenant(ctx)
	if err != nil {
		return err
	}
	if len(preferences) == 0 {
		return apperr.ErrNotifyInvalid
	}
	for _, pref := range preferences {
		if strings.TrimSpace(pref.Type) == "" {
			return apperr.ErrNotifyInvalid
		}
		tpl, err := s.store.GetTemplate(ctx, pref.Type)
		if err != nil {
			return err
		}
		if tpl.Force && !pref.Enabled {
			return apperr.ErrNotifyPreferenceLocked
		}
	}
	return s.store.UpsertPreferences(ctx, id.TenantID, id.AccountID, preferences)
}

// CreateAnnouncement 发布系统公告,公告只写一条,不复制为站内信。
func (s *Service) CreateAnnouncement(ctx context.Context, req AnnouncementRequest) (AnnouncementDTO, error) {
	id, err := currentTenant(ctx)
	if err != nil {
		return AnnouncementDTO{}, err
	}
	if strings.TrimSpace(req.Title) == "" || strings.TrimSpace(req.Content) == "" {
		return AnnouncementDTO{}, apperr.ErrNotifyAnnouncementInvalid
	}
	return s.store.CreateAnnouncement(ctx, s.nextID(), id.AccountID, req)
}

// ListAnnouncements 查询当前用户可见公告并带本人已读状态。
func (s *Service) ListAnnouncements(ctx context.Context, roles []int16) ([]AnnouncementDTO, error) {
	id, err := currentTenant(ctx)
	if err != nil {
		return nil, err
	}
	return s.store.ListAnnouncements(ctx, id.TenantID, id.AccountID, roles)
}

// MarkAnnouncementRead 写入当前用户公告已读状态。
func (s *Service) MarkAnnouncementRead(ctx context.Context, announcementID int64) error {
	id, err := currentTenant(ctx)
	if err != nil {
		return err
	}
	if announcementID <= 0 {
		return apperr.ErrNotifyAnnouncementNotFound
	}
	if _, err := s.store.GetAnnouncement(ctx, id.TenantID, announcementID); err != nil {
		return err
	}
	return s.store.MarkAnnouncementRead(ctx, id.TenantID, id.AccountID, announcementID, s.nextID())
}

// pushUnread 增加未读计数并推送个人红点 topic。
func (s *Service) pushUnread(ctx context.Context, tenantID, receiverID int64) error {
	if s.unread == nil {
		return nil
	}
	count, err := s.unread.Increment(ctx, tenantID, receiverID)
	if err != nil {
		return apperr.ErrNotifySendFailed.WithCause(err)
	}
	if s.broadcaster == nil {
		return nil
	}
	payload := map[string]any{"topic": fmt.Sprintf("notify:%d", receiverID), "payload": map[string]any{"unread_count": count}}
	if err := s.broadcaster.Broadcast(tenantTopic(tenantID, fmt.Sprintf("notify:%d", receiverID)), payload); err != nil {
		return apperr.ErrNotifyPushFailed.WithCause(err)
	}
	return nil
}

// refreshUnreadDot 用 notification 权威状态重建未读缓存并推送个人红点。
func (s *Service) refreshUnreadDot(ctx context.Context, tenantID, accountID int64) {
	if s.unread == nil {
		return
	}
	count, err := s.store.CountUnreadNotifications(ctx, tenantID, accountID)
	if err != nil {
		logging.ErrorContext(ctx, "通知未读计数重建失败", err.Error(),
			slog.Int64("tenant_id", tenantID),
			slog.Int64("account_id", accountID),
		)
		return
	}
	if err := s.unread.Set(ctx, tenantID, accountID, count); err != nil {
		logging.ErrorContext(ctx, "通知未读计数缓存刷新失败", err.Error(),
			slog.Int64("tenant_id", tenantID),
			slog.Int64("account_id", accountID),
		)
		return
	}
	if s.broadcaster == nil {
		return
	}
	payload := map[string]any{"topic": fmt.Sprintf("notify:%d", accountID), "payload": map[string]any{"unread_count": count}}
	if err := s.broadcaster.Broadcast(tenantTopic(tenantID, fmt.Sprintf("notify:%d", accountID)), payload); err != nil {
		logging.ErrorContext(ctx, "通知红点刷新推送失败", err.Error(),
			slog.Int64("tenant_id", tenantID),
			slog.Int64("account_id", accountID),
		)
	}
}

// validateSendRequest 校验内部发送通知请求。
func validateSendRequest(req contracts.NotifySendRequest) error {
	if req.TenantID <= 0 || strings.TrimSpace(req.Type) == "" || len(req.Receivers) == 0 {
		return apperr.ErrNotifyInvalid
	}
	for _, receiverID := range req.Receivers {
		if receiverID <= 0 {
			return apperr.ErrNotifyInvalid
		}
	}
	return nil
}

// renderTemplate 用简单确定性的模板变量替换渲染通知模板。
func renderTemplate(tpl string, params map[string]string) string {
	out := tpl
	for key, value := range params {
		out = strings.ReplaceAll(out, "{{"+key+"}}", value)
	}
	return out
}

// tenantTopic 为业务 topic 加租户隔离前缀。
func tenantTopic(tenantID int64, topic string) string {
	return fmt.Sprintf("tenant:%d:%s", tenantID, topic)
}

// currentTenant 从 context 读取当前租户身份。
func currentTenant(ctx context.Context) (tenant.Identity, error) {
	id, ok := tenant.FromContext(ctx)
	if !ok {
		return tenant.Identity{}, apperr.ErrUnauthorized
	}
	if id.TenantID <= 0 && !id.IsPlatform {
		return tenant.Identity{}, apperr.ErrForbidden
	}
	return id, nil
}

// nextID 生成 M10 自有表主键。
func (s *Service) nextID() int64 {
	return s.idgen.Generate()
}

// hubBroadcaster 把业务层 map 载荷序列化后交给基础 Hub。
type hubBroadcaster struct {
	hub *ws.Hub
}

// newHubBroadcaster 构造 WebSocket Hub 广播适配器。
func newHubBroadcaster(hub *ws.Hub) realtimeBroadcaster {
	return &hubBroadcaster{hub: hub}
}

// Broadcast 序列化实时载荷并广播到 topic。
func (b *hubBroadcaster) Broadcast(topic string, payload map[string]any) error {
	if b.hub == nil {
		return apperr.ErrNotifyPushFailed
	}
	data, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("序列化实时消息失败: %w", err)
	}
	b.hub.Broadcast(topic, data)
	return nil
}
