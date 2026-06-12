// notify service 文件实现 M10 统一通知、公告、未读数和实时推送业务编排。
package notify

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"chaimir/internal/contracts"
	"chaimir/internal/platform/config"
	"chaimir/internal/platform/redis"
	"chaimir/internal/platform/tenant"
	"chaimir/internal/platform/ws"
	"chaimir/pkg/apperr"
	"chaimir/pkg/logging"
	"chaimir/pkg/snowflake"
)

// Service 承载 M10 通知业务编排。
type Service struct {
	store Store
	ids   snowflake.Generator
	redis *redis.Client
	hub   *ws.Hub
	cfg   config.NotifyConfig
}

// ServiceDeps 是 M10 服务装配依赖。
type ServiceDeps struct {
	Store  Store
	IDs    snowflake.Generator
	Redis  *redis.Client
	Hub    *ws.Hub
	Config config.NotifyConfig
}

// NewService 构造 M10 服务。
func NewService(deps ServiceDeps) (*Service, error) {
	if deps.Store == nil || deps.IDs == nil || deps.Redis == nil || deps.Hub == nil {
		return nil, fmt.Errorf("notify service 依赖不完整")
	}
	if deps.Config.UnreadTTLHours <= 0 || deps.Config.SendRateWindowSeconds <= 0 || deps.Config.SendRateMax <= 0 {
		return nil, fmt.Errorf("notify service 配置不完整")
	}
	return &Service{store: deps.Store, ids: deps.IDs, redis: deps.Redis, hub: deps.Hub, cfg: deps.Config}, nil
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
			logging.ErrorContext(ctx, "刷新通知未读数失败", err.Error())
		}
	}
	return nil
}

// Push 向统一 WebSocket topic 推送业务实时消息。
func (s *Service) Push(ctx context.Context, req contracts.NotifyPushRequest) error {
	if req.TenantID <= 0 || strings.TrimSpace(req.Topic) == "" {
		return apperr.ErrNotifyPushFailed
	}
	data, err := json.Marshal(map[string]any{"topic": req.Topic, "payload": req.Payload})
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
	id, err := currentIdentity(ctx)
	if err != nil {
		return AnnouncementDTO{}, err
	}
	if strings.TrimSpace(req.Title) == "" || strings.TrimSpace(req.Content) == "" {
		return AnnouncementDTO{}, apperr.ErrNotifyAnnouncementInvalid
	}
	var out AnnouncementDTO
	err = s.store.TenantTx(ctx, id.TenantID, func(ctx context.Context, tx TxStore) error {
		var err error
		out, err = tx.CreateAnnouncement(ctx, s.ids.Generate(), id.TenantID, id.AccountID, req)
		return err
	})
	return out, err
}

// ListAnnouncements 查询可见公告。
func (s *Service) ListAnnouncements(ctx context.Context, page, size int) ([]AnnouncementDTO, error) {
	id, err := currentIdentity(ctx)
	if err != nil {
		return nil, err
	}
	var out []AnnouncementDTO
	err = s.store.TenantTx(ctx, id.TenantID, func(ctx context.Context, tx TxStore) error {
		var err error
		out, err = tx.ListAnnouncements(ctx, id.TenantID, id.AccountID, page, size)
		return err
	})
	return out, err
}

// MarkAnnouncementRead 标记公告已读。
func (s *Service) MarkAnnouncementRead(ctx context.Context, announcementID int64) error {
	id, err := currentIdentity(ctx)
	if err != nil {
		return err
	}
	return s.store.TenantTx(ctx, id.TenantID, func(ctx context.Context, tx TxStore) error {
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
	data, err := json.Marshal(map[string]any{"type": "unread", "unread": unread})
	if err != nil {
		return err
	}
	s.hub.Broadcast(fmt.Sprintf("notify:%d", accountID), data)
	return nil
}

// countUnread 在租户事务中读取用户未读站内信数量。
func (s *Service) countUnread(ctx context.Context, tenantID, accountID int64) (int64, error) {
	var unread int64
	err := s.store.TenantTx(ctx, tenantID, func(ctx context.Context, tx TxStore) error {
		var err error
		unread, err = tx.CountUnread(ctx, accountID)
		return err
	})
	return unread, err
}

// currentIdentity 读取通知模块要求的租户用户身份。
func currentIdentity(ctx context.Context) (tenant.Identity, error) {
	id, ok := tenant.FromContext(ctx)
	if !ok || id.TenantID <= 0 || id.AccountID <= 0 || id.IsPlatform {
		return tenant.Identity{}, apperr.ErrUnauthorized
	}
	return id, nil
}
