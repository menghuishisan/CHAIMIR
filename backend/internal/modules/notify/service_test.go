// M10 服务测试:覆盖通知模板渲染、偏好强制规则、实时推送与公告已读状态。
package notify

import (
	"context"
	"errors"
	"testing"

	"chaimir/internal/contracts"
	"chaimir/internal/platform/tenant"
	"chaimir/pkg/apperr"
)

// TestSendRendersTemplateAndSkipsDisabledPreference 确认非强制通知遵守用户接收偏好。
func TestSendRendersTemplateAndSkipsDisabledPreference(t *testing.T) {
	store := &fakeNotifyStore{
		template: TemplateDTO{Type: "assignment.due", TitleTemplate: "作业 {{assignment}} 即将截止", ContentTemplate: "{{course}} 的作业将在 {{due}} 截止", Force: false},
		preferences: map[int64]bool{
			1002: false,
		},
	}
	unread := &fakeUnreadCounter{}
	broadcaster := &fakeBroadcaster{}
	svc := &Service{store: store, idgen: fixedIDGen(9001), unread: unread, broadcaster: broadcaster}

	err := svc.Send(testTenantContext(), contracts.NotifySendRequest{
		TenantID: 10,
		Type:     "assignment.due",
		Receivers: []int64{
			1001,
			1002,
		},
		Params: map[string]string{"assignment": "Lab1", "course": "区块链基础", "due": "2026-06-01 23:59"},
		Link:   "/assignments/1",
	})
	if err != nil {
		t.Fatalf("Send returned error: %v", err)
	}
	if len(store.created) != 1 {
		t.Fatalf("expected one delivered notification, got %#v", store.created)
	}
	if store.created[0].ReceiverID != 1001 || store.created[0].Title != "作业 Lab1 即将截止" {
		t.Fatalf("unexpected rendered notification: %#v", store.created[0])
	}
	if unread.incremented[1001] != 1 || unread.incremented[1002] != 0 {
		t.Fatalf("unexpected unread increments: %#v", unread.incremented)
	}
	if unread.incrementTenantID != 10 {
		t.Fatalf("unread counter increment must keep tenant scope, got %d", unread.incrementTenantID)
	}
	if broadcaster.lastTopic != "tenant:10:notify:1001" {
		t.Fatalf("red dot should be pushed to tenant-scoped personal topic, got %s", broadcaster.lastTopic)
	}
}

// TestSendForceTemplateIgnoresDisabledPreference 确认强制模板不被用户偏好屏蔽。
func TestSendForceTemplateIgnoresDisabledPreference(t *testing.T) {
	store := &fakeNotifyStore{
		template:    TemplateDTO{Type: "grade.review", TitleTemplate: "成绩审核结果", ContentTemplate: "{{result}}", Force: true},
		preferences: map[int64]bool{1002: false},
	}
	svc := &Service{store: store, idgen: fixedIDGen(9002), unread: &fakeUnreadCounter{}, broadcaster: &fakeBroadcaster{}}

	err := svc.Send(testTenantContext(), contracts.NotifySendRequest{
		TenantID: 10, Type: "grade.review", Receivers: []int64{1002}, Params: map[string]string{"result": "已通过"},
	})
	if err != nil {
		t.Fatalf("Send returned error: %v", err)
	}
	if len(store.created) != 1 || store.created[0].ReceiverID != 1002 {
		t.Fatalf("force notification should be delivered, got %#v", store.created)
	}
}

// TestSendKeepsNotificationWhenUnreadCounterFails 确认站内信已落库时,未读计数失败不会把强制通知整体判定为失败。
func TestSendKeepsNotificationWhenUnreadCounterFails(t *testing.T) {
	store := &fakeNotifyStore{
		template: TemplateDTO{Type: "grade.review", TitleTemplate: "成绩审核结果", ContentTemplate: "{{result}}", Force: true},
	}
	svc := &Service{
		store:       store,
		idgen:       fixedIDGen(9005),
		unread:      &fakeUnreadCounter{incrementErr: errors.New("redis down")},
		broadcaster: &fakeBroadcaster{},
	}

	err := svc.Send(testTenantContext(), contracts.NotifySendRequest{
		TenantID: 10, Type: "grade.review", Receivers: []int64{1002}, Params: map[string]string{"result": "已通过"},
	})
	if err != nil {
		t.Fatalf("Send should keep notification success when unread counter fails: %v", err)
	}
	if len(store.created) != 1 || store.created[0].ReceiverID != 1002 {
		t.Fatalf("notification row must still be created, got %#v", store.created)
	}
}

// TestSendKeepsNotificationWhenRealtimePushFails 确认实时红点失败时仍保留已投递站内信,由站内信承担兜底。
func TestSendKeepsNotificationWhenRealtimePushFails(t *testing.T) {
	store := &fakeNotifyStore{
		template: TemplateDTO{Type: "grade.review", TitleTemplate: "成绩审核结果", ContentTemplate: "{{result}}", Force: true},
	}
	svc := &Service{
		store:       store,
		idgen:       fixedIDGen(9006),
		unread:      &fakeUnreadCounter{},
		broadcaster: &fakeBroadcaster{broadcastErr: errors.New("ws down")},
	}

	err := svc.Send(testTenantContext(), contracts.NotifySendRequest{
		TenantID: 10, Type: "grade.review", Receivers: []int64{1002}, Params: map[string]string{"result": "已通过"},
	})
	if err != nil {
		t.Fatalf("Send should keep notification success when realtime push fails: %v", err)
	}
	if len(store.created) != 1 || store.created[0].ReceiverID != 1002 {
		t.Fatalf("notification row must still be created, got %#v", store.created)
	}
}

// TestUpdatePreferencesRejectsForceTemplate 确认必要通知不能被关闭。
func TestUpdatePreferencesRejectsForceTemplate(t *testing.T) {
	store := &fakeNotifyStore{template: TemplateDTO{Type: "system.maintenance", Force: true}}
	svc := &Service{store: store, idgen: fixedIDGen(9003)}

	err := svc.UpdatePreferences(testTenantContext(), []PreferenceRequest{{Type: "system.maintenance", Enabled: false}})
	if err == nil {
		t.Fatalf("expected force preference rejection")
	}
	if ae, ok := apperr.As(err); !ok || ae.Code != apperr.ErrNotifyPreferenceLocked.Code {
		t.Fatalf("expected notify preference locked error, got %v", err)
	}
}

// TestPushBroadcastsTenantScopedEnvelope 确认实时推送按租户隔离 topic,且不解释业务 payload。
func TestPushBroadcastsTenantScopedEnvelope(t *testing.T) {
	broadcaster := &fakeBroadcaster{}
	svc := &Service{broadcaster: broadcaster}

	err := svc.Push(testTenantContext(), contracts.NotifyPushRequest{
		TenantID: 10,
		Topic:    "contest:55:leaderboard",
		Payload:  map[string]any{"rank": []int{1, 2, 3}},
	})
	if err != nil {
		t.Fatalf("Push returned error: %v", err)
	}
	if broadcaster.lastTopic != "tenant:10:contest:55:leaderboard" {
		t.Fatalf("unexpected tenant-scoped topic: %s", broadcaster.lastTopic)
	}
	if broadcaster.lastPayload["topic"] != "contest:55:leaderboard" {
		t.Fatalf("external topic should remain unchanged in envelope: %#v", broadcaster.lastPayload)
	}
}

// TestUnreadCountRebuildsCacheFromStore 确认未读缓存缺失时会按 notification 权威状态重建,而不是把未读数误报为 0。
func TestUnreadCountRebuildsCacheFromStore(t *testing.T) {
	store := &fakeNotifyStore{unreadCount: 3}
	unread := &fakeUnreadCounter{cached: false}
	svc := &Service{store: store, unread: unread}

	count, err := svc.UnreadCount(testTenantContext())
	if err != nil {
		t.Fatalf("UnreadCount returned error: %v", err)
	}
	if count != 3 {
		t.Fatalf("expected rebuilt unread count 3, got %d", count)
	}
	if store.countUnreadCalls != 1 {
		t.Fatalf("expected one authoritative unread query, got %d", store.countUnreadCalls)
	}
	if unread.setCount != 3 || unread.setCalls != 1 {
		t.Fatalf("expected unread cache rebuild to persist count, got count=%d calls=%d", unread.setCount, unread.setCalls)
	}
}

// TestMarkAllNotificationsReadKeepsSuccessWhenCacheResetFails 确认批量已读以站内信权威状态为准,缓存清理失败不会把已完成操作伪装成失败。
func TestMarkAllNotificationsReadKeepsSuccessWhenCacheResetFails(t *testing.T) {
	store := &fakeNotifyStore{}
	unread := &fakeUnreadCounter{resetErr: errors.New("redis reset failed")}
	svc := &Service{store: store, unread: unread}

	err := svc.MarkAllNotificationsRead(testTenantContext())
	if err != nil {
		t.Fatalf("MarkAllNotificationsRead should keep success after cache reset failure: %v", err)
	}
	if store.readAllAccountID != 501 {
		t.Fatalf("expected notification rows to be marked read for account 501, got %d", store.readAllAccountID)
	}
}

// TestMarkNotificationReadRefreshesUnreadDot 确认单条已读后按站内信权威状态刷新缓存并推送红点。
func TestMarkNotificationReadRefreshesUnreadDot(t *testing.T) {
	store := &fakeNotifyStore{unreadCount: 2}
	unread := &fakeUnreadCounter{}
	broadcaster := &fakeBroadcaster{}
	svc := &Service{store: store, unread: unread, broadcaster: broadcaster}

	err := svc.MarkNotificationRead(testTenantContext(), 7001)
	if err != nil {
		t.Fatalf("MarkNotificationRead returned error: %v", err)
	}
	if store.readNotificationID != 7001 {
		t.Fatalf("expected notification 7001 to be marked read, got %d", store.readNotificationID)
	}
	if store.countUnreadCalls != 1 || unread.setCount != 2 || unread.setCalls != 1 {
		t.Fatalf("expected unread cache refresh from store, calls=%d set=%d setCalls=%d", store.countUnreadCalls, unread.setCount, unread.setCalls)
	}
	if broadcaster.lastTopic != "tenant:10:notify:501" {
		t.Fatalf("expected refreshed red dot push, got %s", broadcaster.lastTopic)
	}
}

// TestDeleteNotificationRefreshesUnreadDot 确认软删站内信后未读红点不会继续使用旧缓存。
func TestDeleteNotificationRefreshesUnreadDot(t *testing.T) {
	store := &fakeNotifyStore{unreadCount: 1}
	unread := &fakeUnreadCounter{}
	broadcaster := &fakeBroadcaster{}
	svc := &Service{store: store, unread: unread, broadcaster: broadcaster}

	err := svc.DeleteNotification(testTenantContext(), 7002)
	if err != nil {
		t.Fatalf("DeleteNotification returned error: %v", err)
	}
	if store.deletedID != 7002 {
		t.Fatalf("expected notification 7002 to be deleted, got %d", store.deletedID)
	}
	if store.countUnreadCalls != 1 || unread.setCount != 1 || unread.setCalls != 1 {
		t.Fatalf("expected unread cache refresh from store, calls=%d set=%d setCalls=%d", store.countUnreadCalls, unread.setCount, unread.setCalls)
	}
	if broadcaster.lastTopic != "tenant:10:notify:501" {
		t.Fatalf("expected refreshed red dot push, got %s", broadcaster.lastTopic)
	}
}

// TestAnnouncementReadIsTenantScoped 确认公告已读状态按租户和账号写入,不写放大。
func TestAnnouncementReadIsTenantScoped(t *testing.T) {
	store := &fakeNotifyStore{announcement: AnnouncementDTO{ID: "7001", TenantID: "10", Title: "维护通知", Scope: AnnouncementScopeTenant}}
	svc := &Service{store: store, idgen: fixedIDGen(9004)}

	if err := svc.MarkAnnouncementRead(testTenantContext(), 7001); err != nil {
		t.Fatalf("MarkAnnouncementRead returned error: %v", err)
	}
	if store.readAnnouncementID != 7001 || store.readAccountID != 501 || store.readTenantID != 10 {
		t.Fatalf("announcement read should be tenant scoped, got tenant=%d account=%d announcement=%d", store.readTenantID, store.readAccountID, store.readAnnouncementID)
	}
	if len(store.created) != 0 {
		t.Fatalf("announcement read must not create notification rows")
	}
}

// TestUnreadKeyIncludesTenantScope 确认 Redis 未读缓存键与接口语义一致,不会丢弃租户范围。
func TestUnreadKeyIncludesTenantScope(t *testing.T) {
	if got := unreadKey(10, 501); got != "tenant:10:unread:501" {
		t.Fatalf("unexpected unread key: %s", got)
	}
}

// testTenantContext 构造 M10 服务测试租户上下文。
func testTenantContext() context.Context {
	return tenant.WithContext(context.Background(), tenant.Identity{TenantID: 10, AccountID: 501})
}

type fixedIDGen int64

// Generate 返回固定雪花 ID,便于断言服务写入。
func (g fixedIDGen) Generate() int64 { return int64(g) }

type fakeNotifyStore struct {
	template           TemplateDTO
	announcement       AnnouncementDTO
	preferences        map[int64]bool
	unreadCount        int64
	countUnreadCalls   int
	created            []NotificationCreate
	savedPreferences   []PreferenceRequest
	readNotificationID int64
	readAllAccountID   int64
	deletedID          int64
	readTenantID       int64
	readAccountID      int64
	readAnnouncementID int64
}

func (f *fakeNotifyStore) GetTemplate(context.Context, string) (TemplateDTO, error) {
	return f.template, nil
}

func (f *fakeNotifyStore) GetPreference(_ context.Context, _ int64, accountID int64, _ string) (bool, bool, error) {
	if f.preferences == nil {
		return true, false, nil
	}
	enabled, ok := f.preferences[accountID]
	return enabled, ok, nil
}

func (f *fakeNotifyStore) CreateNotification(_ context.Context, row NotificationCreate) error {
	f.created = append(f.created, row)
	return nil
}

func (f *fakeNotifyStore) ListInbox(context.Context, int64, int64, InboxQuery) ([]NotificationDTO, int64, error) {
	return nil, 0, nil
}

func (f *fakeNotifyStore) CountUnreadNotifications(context.Context, int64, int64) (int64, error) {
	f.countUnreadCalls++
	return f.unreadCount, nil
}

func (f *fakeNotifyStore) MarkNotificationRead(_ context.Context, _ int64, notificationID int64) error {
	f.readNotificationID = notificationID
	return nil
}

func (f *fakeNotifyStore) MarkAllNotificationsRead(_ context.Context, accountID int64) error {
	f.readAllAccountID = accountID
	return nil
}

func (f *fakeNotifyStore) SoftDeleteNotification(_ context.Context, _ int64, notificationID int64) error {
	f.deletedID = notificationID
	return nil
}

func (f *fakeNotifyStore) ListPreferences(context.Context, int64, int64) ([]PreferenceDTO, error) {
	return nil, nil
}

func (f *fakeNotifyStore) UpsertPreferences(_ context.Context, _ int64, _ int64, preferences []PreferenceRequest) error {
	f.savedPreferences = preferences
	return nil
}

func (f *fakeNotifyStore) CreateAnnouncement(context.Context, int64, int64, AnnouncementRequest) (AnnouncementDTO, error) {
	return f.announcement, nil
}

func (f *fakeNotifyStore) ListAnnouncements(context.Context, int64, int64, []int16) ([]AnnouncementDTO, error) {
	return []AnnouncementDTO{f.announcement}, nil
}

func (f *fakeNotifyStore) GetAnnouncement(context.Context, int64, int64) (AnnouncementDTO, error) {
	return f.announcement, nil
}

func (f *fakeNotifyStore) MarkAnnouncementRead(_ context.Context, tenantID, accountID, announcementID, _ int64) error {
	f.readTenantID = tenantID
	f.readAccountID = accountID
	f.readAnnouncementID = announcementID
	return nil
}

type fakeUnreadCounter struct {
	incremented       map[int64]int64
	incrementTenantID int64
	incrementErr      error
	count             int64
	cached            bool
	countErr          error
	setCount          int64
	setCalls          int
	setErr            error
	resetErr          error
}

func (f *fakeUnreadCounter) Increment(_ context.Context, tenantID, accountID int64) (int64, error) {
	if f.incrementErr != nil {
		return 0, f.incrementErr
	}
	f.incrementTenantID = tenantID
	if f.incremented == nil {
		f.incremented = make(map[int64]int64)
	}
	f.incremented[accountID]++
	return f.incremented[accountID], nil
}

func (f *fakeUnreadCounter) Get(context.Context, int64, int64) (int64, bool, error) {
	if f.countErr != nil {
		return 0, false, f.countErr
	}
	return f.count, f.cached, nil
}

func (f *fakeUnreadCounter) Set(_ context.Context, _ int64, _ int64, count int64) error {
	if f.setErr != nil {
		return f.setErr
	}
	f.setCalls++
	f.setCount = count
	return nil
}

func (f *fakeUnreadCounter) Reset(context.Context, int64, int64) error {
	return f.resetErr
}

type fakeBroadcaster struct {
	lastTopic    string
	lastPayload  map[string]any
	broadcastErr error
}

func (f *fakeBroadcaster) Broadcast(topic string, payload map[string]any) error {
	if f.broadcastErr != nil {
		return f.broadcastErr
	}
	f.lastTopic = topic
	f.lastPayload = payload
	return nil
}
