// notify service 文件实现 M10 统一通知、公告、未读数和实时推送业务编排。
package notify

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"chaimir/internal/contracts"
	"chaimir/internal/platform/config"
	"chaimir/internal/platform/jsonx"
	"chaimir/internal/platform/tenant"
	"chaimir/internal/platform/timex"
	"chaimir/internal/platform/ws"
	"chaimir/pkg/apperr"
	"chaimir/pkg/logging"
	"chaimir/pkg/snowflake"
)

// Service 承载 M10 通知业务编排。
type Service struct {
	store Store
	ids   snowflake.Generator
	redis Cache
	hub   *ws.Hub
	roles RoleReader
	cfg   config.NotifyConfig
}

// Cache 定义 M10 使用的 Redis 缓存和限频能力。
type Cache interface {
	GetInt64(ctx context.Context, key string) (int64, bool, error)
	SetInt64(ctx context.Context, key string, value int64, ttl time.Duration) error
	Delete(ctx context.Context, key string) error
	IncrWithTTL(ctx context.Context, key string, ttl time.Duration) (int64, error)
}

// RoleReader 定义 M10 使用的账号角色只读契约。
type RoleReader interface {
	HasRole(ctx context.Context, accountID int64, role string) (bool, error)
}

// ServiceDeps 是 M10 服务装配依赖。
type ServiceDeps struct {
	Store  Store
	IDs    snowflake.Generator
	Redis  Cache
	Hub    *ws.Hub
	Roles  RoleReader
	Config config.NotifyConfig
}

// NewService 构造 M10 服务。
func NewService(deps ServiceDeps) (*Service, error) {
	if deps.Store == nil || deps.IDs == nil || deps.Redis == nil || deps.Hub == nil || deps.Roles == nil {
		return nil, fmt.Errorf("notify service 依赖不完整")
	}
	if deps.Config.UnreadTTLHours <= 0 || deps.Config.SendRateWindowSeconds <= 0 || deps.Config.SendRateMax <= 0 {
		return nil, fmt.Errorf("notify service 配置不完整")
	}
	return &Service{store: deps.Store, ids: deps.IDs, redis: deps.Redis, hub: deps.Hub, roles: deps.Roles, cfg: deps.Config}, nil
}

// Send 渲染模板并按接收人偏好写入站内信。
func (s *Service) Send(ctx context.Context, req contracts.NotifySendRequest) error {
	input, err := validateSendRequest(SendRequest{TenantID: req.TenantID, Type: req.Type, Receivers: req.Receivers, Params: req.Params, Link: req.Link})
	if err != nil {
		return err
	}
	if err := s.checkRateLimit(ctx, input.TenantID, input.Type); err != nil {
		return err
	}
	var delivered []int64
	err = s.store.TenantTx(ctx, input.TenantID, func(ctx context.Context, tx TxStore) error {
		tpl, err := tx.GetNotificationTemplate(ctx, input.Type)
		if err != nil {
			return apperr.ErrNotifyTemplateUnavailable.WithCause(err)
		}
		rows := make([]notificationRecord, 0, len(input.Receivers))
		for _, receiverID := range input.Receivers {
			enabled := true
			if !tpl.Force {
				enabled, err = tx.PreferenceEnabled(ctx, input.TenantID, receiverID, input.Type)
				if err != nil {
					return apperr.ErrNotifySendFailed.WithCause(err)
				}
			}
			if !enabled {
				continue
			}
			rows = append(rows, notificationRecord{ID: s.ids.Generate(), TenantID: input.TenantID, ReceiverID: receiverID, Type: input.Type, Title: renderTemplate(tpl.TitleTpl, input.Params), Content: renderTemplate(tpl.ContentTpl, input.Params), Link: input.Link})
			delivered = append(delivered, receiverID)
		}
		if len(rows) == 0 {
			return nil
		}
		return tx.CreateNotifications(ctx, rows)
	})
	if err != nil {
		return err
	}
	for _, receiverID := range delivered {
		if err := s.refreshUnread(ctx, input.TenantID, receiverID); err != nil {
			logging.ErrorContext(ctx, "刷新通知未读数失败", err.Error(), slog.Int64("tenant_id", input.TenantID), slog.Int64("receiver_id", receiverID))
		}
	}
	return nil
}

// Push 向统一 WebSocket topic 推送业务实时消息。
func (s *Service) Push(ctx context.Context, req contracts.NotifyPushRequest) error {
	if req.TenantID <= 0 || strings.TrimSpace(req.Topic) == "" {
		return apperr.ErrNotifyPushFailed
	}
	if err := ValidatePushTopic(req.TenantID, req.Topic); err != nil {
		return err
	}
	data, err := jsonx.AnyBytes(map[string]any{"topic": req.Topic, "payload": req.Payload}, apperr.ErrNotifyPushFailed)
	if err != nil {
		return apperr.ErrNotifyPushFailed.WithCause(err)
	}
	s.hub.Broadcast(req.Topic, data)
	return nil
}

// Inbox 查询当前用户站内信。
func (s *Service) Inbox(ctx context.Context, isRead *bool, typ string, page, size int) ([]NotificationDTO, int64, error) {
	id, err := currentIdentity(ctx)
	if err != nil {
		return nil, 0, err
	}
	var out []NotificationDTO
	var total int64
	err = s.store.TenantTx(ctx, id.TenantID, func(ctx context.Context, tx TxStore) error {
		var err error
		out, total, err = tx.ListNotifications(ctx, id.AccountID, isRead, typ, page, size)
		return err
	})
	if err != nil {
		return nil, 0, apperr.ErrNotifyInboxQueryInvalid.WithCause(err)
	}
	return out, total, nil
}

// Unread 查询当前用户未读数。
func (s *Service) Unread(ctx context.Context) (UnreadDTO, error) {
	id, err := currentIdentity(ctx)
	if err != nil {
		return UnreadDTO{}, err
	}
	n, err := s.countUnread(ctx, id.TenantID, id.AccountID)
	if err != nil {
		return UnreadDTO{}, apperr.ErrNotifyInboxQueryInvalid.WithCause(err)
	}
	return UnreadDTO{Unread: n}, nil
}

// MarkRead 标记单条站内信已读。
func (s *Service) MarkRead(ctx context.Context, notificationID int64) (NotificationDTO, error) {
	id, err := currentIdentity(ctx)
	if err != nil {
		return NotificationDTO{}, err
	}
	var out NotificationDTO
	err = s.store.TenantTx(ctx, id.TenantID, func(ctx context.Context, tx TxStore) error {
		var err error
		out, err = tx.MarkNotificationRead(ctx, notificationID, id.AccountID)
		return err
	})
	if err != nil {
		return NotificationDTO{}, apperr.ErrNotifyInboxNotFound.WithCause(err)
	}
	if err := s.invalidateUnread(ctx, id.TenantID, id.AccountID); err != nil {
		return NotificationDTO{}, apperr.ErrNotifyInboxQueryInvalid.WithCause(err)
	}
	if err := s.refreshUnread(ctx, id.TenantID, id.AccountID); err != nil {
		return NotificationDTO{}, apperr.ErrNotifyInboxQueryInvalid.WithCause(err)
	}
	return out, nil
}

// MarkAllRead 标记当前用户全部站内信已读。
func (s *Service) MarkAllRead(ctx context.Context) error {
	id, err := currentIdentity(ctx)
	if err != nil {
		return err
	}
	if err := s.store.TenantTx(ctx, id.TenantID, func(ctx context.Context, tx TxStore) error {
		return tx.MarkAllNotificationsRead(ctx, id.AccountID)
	}); err != nil {
		return apperr.ErrNotifyInboxQueryInvalid.WithCause(err)
	}
	if err := s.invalidateUnread(ctx, id.TenantID, id.AccountID); err != nil {
		return apperr.ErrNotifyInboxQueryInvalid.WithCause(err)
	}
	return s.refreshUnread(ctx, id.TenantID, id.AccountID)
}

// DeleteNotification 删除当前用户站内信。
func (s *Service) DeleteNotification(ctx context.Context, notificationID int64) (NotificationDTO, error) {
	id, err := currentIdentity(ctx)
	if err != nil {
		return NotificationDTO{}, err
	}
	var out NotificationDTO
	err = s.store.TenantTx(ctx, id.TenantID, func(ctx context.Context, tx TxStore) error {
		var err error
		out, err = tx.DeleteNotification(ctx, notificationID, id.AccountID)
		return err
	})
	if err != nil {
		return NotificationDTO{}, apperr.ErrNotifyInboxNotFound.WithCause(err)
	}
	if err := s.invalidateUnread(ctx, id.TenantID, id.AccountID); err != nil {
		return NotificationDTO{}, apperr.ErrNotifyInboxQueryInvalid.WithCause(err)
	}
	if err := s.refreshUnread(ctx, id.TenantID, id.AccountID); err != nil {
		return NotificationDTO{}, apperr.ErrNotifyInboxQueryInvalid.WithCause(err)
	}
	return out, nil
}

// ListPreferences 查询当前用户通知偏好。
func (s *Service) ListPreferences(ctx context.Context) ([]PreferenceDTO, error) {
	id, err := currentIdentity(ctx)
	if err != nil {
		return nil, err
	}
	var out []PreferenceDTO
	err = s.store.TenantTx(ctx, id.TenantID, func(ctx context.Context, tx TxStore) error {
		var err error
		out, err = tx.ListPreferences(ctx, id.AccountID)
		return err
	})
	return out, err
}

// UpsertPreference 设置当前用户通知偏好。
func (s *Service) UpsertPreference(ctx context.Context, req PreferenceRequest) (PreferenceDTO, error) {
	id, err := currentIdentity(ctx)
	if err != nil {
		return PreferenceDTO{}, err
	}
	req.Type = strings.TrimSpace(req.Type)
	if req.Type == "" {
		return PreferenceDTO{}, apperr.ErrNotifyRequestInvalid
	}
	var out PreferenceDTO
	err = s.store.TenantTx(ctx, id.TenantID, func(ctx context.Context, tx TxStore) error {
		tpl, err := tx.GetNotificationTemplate(ctx, req.Type)
		if err != nil {
			return apperr.ErrNotifyTemplateUnavailable.WithCause(err)
		}
		if tpl.Force && !req.Enabled {
			return apperr.ErrNotifyPreferenceLocked
		}
		out, err = tx.UpsertPreference(ctx, s.ids.Generate(), id.TenantID, id.AccountID, req.Type, req.Enabled)
		return err
	})
	return out, err
}

// CreateAnnouncement 发布系统公告。
func (s *Service) CreateAnnouncement(ctx context.Context, req AnnouncementRequest) (AnnouncementDTO, error) {
	id, err := currentIdentityAllowPlatform(ctx)
	if err != nil {
		return AnnouncementDTO{}, err
	}
	if err := validateAnnouncementRequest(req, id.IsPlatform); err != nil {
		return AnnouncementDTO{}, err
	}
	tenantID := id.TenantID
	if req.Scope == AnnouncementScopePlatform {
		tenantID = 0
	}
	var out AnnouncementDTO
	write := func(ctx context.Context, tx TxStore) error {
		var err error
		out, err = tx.CreateAnnouncement(ctx, s.ids.Generate(), tenantID, id.AccountID, req)
		return err
	}
	if tenantID == 0 {
		err = s.store.PlatformTx(ctx, write)
	} else {
		err = s.store.TenantTx(ctx, tenantID, write)
	}
	return out, err
}

// ListAnnouncements 查询可见公告。
func (s *Service) ListAnnouncements(ctx context.Context, page, size int) ([]AnnouncementDTO, error) {
	id, err := currentIdentity(ctx)
	if err != nil {
		return nil, err
	}
	var out []AnnouncementDTO
	roleNumbers, err := s.currentRoleNumbers(ctx, id.AccountID)
	if err != nil {
		return nil, err
	}
	err = s.store.TenantTx(ctx, id.TenantID, func(ctx context.Context, tx TxStore) error {
		var err error
		out, err = tx.ListAnnouncements(ctx, id.TenantID, id.AccountID, roleNumbers, page, size)
		return err
	})
	if err != nil {
		return nil, err
	}
	return s.filterAnnouncementsByRole(ctx, id.AccountID, out)
}

// MarkAnnouncementRead 标记公告已读。
func (s *Service) MarkAnnouncementRead(ctx context.Context, announcementID int64) error {
	id, err := currentIdentity(ctx)
	if err != nil {
		return err
	}
	roleNumbers, err := s.currentRoleNumbers(ctx, id.AccountID)
	if err != nil {
		return err
	}
	return s.store.TenantTx(ctx, id.TenantID, func(ctx context.Context, tx TxStore) error {
		if _, err := tx.GetVisibleAnnouncement(ctx, id.TenantID, id.AccountID, roleNumbers, announcementID); err != nil {
			return apperr.ErrNotifyAnnouncementNotFound.WithCause(err)
		}
		return tx.MarkAnnouncementRead(ctx, s.ids.Generate(), id.TenantID, announcementID, id.AccountID)
	})
}

// HandleSubscribe 处理交互式 WebSocket 订阅。
func (s *Service) HandleSubscribe(ctx context.Context, conn *ws.Conn) error {
	id, err := currentIdentity(ctx)
	if err != nil {
		return err
	}
	if err := conn.BindSession(ws.SessionKey{TenantID: id.TenantID, AccountID: id.AccountID}); err != nil {
		return apperr.ErrNotifyChannelUnavailable.WithCause(err)
	}
	s.hub.Subscribe(conn, fmt.Sprintf("notify:%d", id.AccountID))
	for {
		var msg SubscribeMessage
		if err := conn.ReadJSON(&msg); err != nil {
			return nil
		}
		if strings.TrimSpace(msg.Action) != "subscribe" || len(msg.Topics) == 0 {
			return apperr.ErrNotifySubscribeInvalid
		}
		for _, topic := range msg.Topics {
			if err := AuthorizeTopic(id.TenantID, id.AccountID, topic); err != nil {
				return err
			}
			s.hub.Subscribe(conn, strings.TrimSpace(topic))
		}
		if err := conn.SendJSON(map[string]any{"type": "subscribed", "topics": msg.Topics}); err != nil {
			return apperr.ErrNotifyChannelUnavailable.WithCause(err)
		}
	}
}

// CloseSession 关闭指定用户实时连接。
func (s *Service) CloseSession(ctx context.Context, tenantID, accountID int64) error {
	return s.hub.CloseSession(ws.SessionKey{TenantID: tenantID, AccountID: accountID})
}

// RunCleanupOnce 执行一次站内信过期清理任务。
func (s *Service) RunCleanupOnce(ctx context.Context) error {
	cutoff := timex.Now().Add(-time.Duration(s.cfg.RetentionDays) * 24 * time.Hour)
	return s.store.PlatformTx(ctx, func(ctx context.Context, tx TxStore) error {
		return tx.DeleteExpiredNotifications(ctx, cutoff)
	})
}

// checkRateLimit 使用 Redis 窗口计数限制通知发送频率。
func (s *Service) checkRateLimit(ctx context.Context, tenantID int64, typ string) error {
	key := fmt.Sprintf("tenant:%d:notify:send:%s", tenantID, strings.ToLower(strings.TrimSpace(typ)))
	count, err := s.redis.IncrWithTTL(ctx, key, time.Duration(s.cfg.SendRateWindowSeconds)*time.Second)
	if err != nil {
		return apperr.ErrNotifySendFailed.WithCause(err)
	}
	if count > int64(s.cfg.SendRateMax) {
		return apperr.ErrNotifyRateLimited
	}
	return nil
}

// refreshUnread 重算未读数并通过统一实时通道推送给用户。
func (s *Service) refreshUnread(ctx context.Context, tenantID, accountID int64) error {
	unread, err := s.countUnread(ctx, tenantID, accountID)
	if err != nil {
		return err
	}
	data, err := jsonx.AnyBytes(map[string]any{"type": "unread", "unread": unread}, apperr.ErrNotifyInboxQueryInvalid)
	if err != nil {
		return err
	}
	s.hub.Broadcast(fmt.Sprintf("notify:%d", accountID), data)
	return nil
}

// invalidateUnread 删除未读缓存,确保后续重建读取权威站内信状态。
func (s *Service) invalidateUnread(ctx context.Context, tenantID, accountID int64) error {
	return s.redis.Delete(ctx, unreadKey(tenantID, accountID))
}

// countUnread 在租户事务中读取用户未读站内信数量。
func (s *Service) countUnread(ctx context.Context, tenantID, accountID int64) (int64, error) {
	key := unreadKey(tenantID, accountID)
	if unread, ok, err := s.redis.GetInt64(ctx, key); err != nil {
		return 0, err
	} else if ok {
		return unread, nil
	}
	var unread int64
	err := s.store.TenantTx(ctx, tenantID, func(ctx context.Context, tx TxStore) error {
		var err error
		unread, err = tx.CountUnread(ctx, accountID)
		return err
	})
	if err != nil {
		return 0, err
	}
	if err := s.redis.SetInt64(ctx, key, unread, time.Duration(s.cfg.UnreadTTLHours)*time.Hour); err != nil {
		return 0, err
	}
	return unread, nil
}

// unreadKey 生成 M10 未读缓存键。
func unreadKey(tenantID, accountID int64) string {
	return fmt.Sprintf("tenant:%d:unread:%d", tenantID, accountID)
}

// currentIdentity 读取通知模块要求的租户用户身份。
func currentIdentity(ctx context.Context) (tenant.Identity, error) {
	id, ok := tenant.FromContext(ctx)
	if !ok || id.TenantID <= 0 || id.AccountID <= 0 || id.IsPlatform {
		return tenant.Identity{}, apperr.ErrUnauthorized
	}
	return id, nil
}

// currentIdentityAllowPlatform 读取公告发布需要的租户或平台身份。
func currentIdentityAllowPlatform(ctx context.Context) (tenant.Identity, error) {
	id, ok := tenant.FromContext(ctx)
	if !ok || id.AccountID <= 0 {
		return tenant.Identity{}, apperr.ErrUnauthorized
	}
	if !id.IsPlatform && id.TenantID <= 0 {
		return tenant.Identity{}, apperr.ErrUnauthorized
	}
	return id, nil
}

// filterAnnouncementsByRole 按指定角色公告的目标角色过滤用户可见列表。
func (s *Service) filterAnnouncementsByRole(ctx context.Context, accountID int64, items []AnnouncementDTO) ([]AnnouncementDTO, error) {
	out := make([]AnnouncementDTO, 0, len(items))
	for _, item := range items {
		if item.Scope != AnnouncementScopeRoles {
			out = append(out, item)
			continue
		}
		allowed := false
		for _, roleNum := range item.TargetRoles {
			role := contracts.RoleCode(roleNum)
			if role == "unknown" {
				continue
			}
			has, err := s.roles.HasRole(ctx, accountID, role)
			if err != nil {
				return nil, apperr.ErrNotifyAnnouncementNotFound.WithCause(err)
			}
			if has {
				allowed = true
				break
			}
		}
		if allowed {
			out = append(out, item)
		}
	}
	return out, nil
}

// currentRoleNumbers 读取当前用户角色并转成公告查询所需的数字角色集合。
func (s *Service) currentRoleNumbers(ctx context.Context, accountID int64) ([]int16, error) {
	candidates := []string{contracts.RoleSchoolAdmin, contracts.RoleTeacher, contracts.RoleStudent}
	out := make([]int16, 0, len(candidates))
	for _, role := range candidates {
		has, err := s.roles.HasRole(ctx, accountID, role)
		if err != nil {
			return nil, apperr.ErrNotifyAnnouncementNotFound.WithCause(err)
		}
		if !has {
			continue
		}
		if n, ok := contracts.RoleNumber(role); ok {
			out = append(out, n)
		}
	}
	if len(out) == 0 {
		out = []int16{-1}
	}
	return out, nil
}
