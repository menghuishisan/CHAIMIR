// contest service 文件定义 M8 服务依赖注入和通用业务编排,不接收数据库连接。
package contest

import (
	"context"
	"fmt"

	"chaimir/internal/contracts"
	"chaimir/internal/platform/audit"
	"chaimir/internal/platform/config"
	"chaimir/internal/platform/eventbus"
	"chaimir/internal/platform/tenant"
	"chaimir/pkg/apperr"
	"chaimir/pkg/crypto"
	"chaimir/pkg/snowflake"
)

// Service 承载 contest 模块业务编排,依赖 repo 接口和跨模块 contracts。
type Service struct {
	store         Store
	ids           snowflake.Generator
	cfg           config.ContestConfig
	audit         audit.Writer
	roles         contracts.IdentityService
	content       contracts.ContentReadService
	contentImport contracts.ContentImportService
	sandbox       contracts.SandboxService
	judge         contracts.JudgeService
	fingerprint   contracts.FingerprintService
	notify        contracts.NotifyService
	bus           eventbus.Bus
	cipher        *crypto.Cipher
}

// ServiceDeps 是 contest service 的装配依赖集合。
type ServiceDeps struct {
	Store         Store
	IDs           snowflake.Generator
	Config        config.ContestConfig
	Audit         audit.Writer
	Roles         contracts.IdentityService
	Content       contracts.ContentReadService
	ContentImport contracts.ContentImportService
	Sandbox       contracts.SandboxService
	Judge         contracts.JudgeService
	Fingerprint   contracts.FingerprintService
	Notify        contracts.NotifyService
	Bus           eventbus.Bus
	Cipher        *crypto.Cipher
}

// NewService 构造 contest 服务,不接收数据库连接,由装配层传入 Store。
func NewService(deps ServiceDeps) (*Service, error) {
	if deps.Store == nil {
		return nil, fmt.Errorf("contest service 缺少 store")
	}
	if deps.IDs == nil {
		return nil, fmt.Errorf("contest service 缺少 ID 生成器")
	}
	if deps.Audit == nil {
		return nil, fmt.Errorf("contest service 缺少审计写入器")
	}
	if deps.Roles == nil {
		return nil, fmt.Errorf("contest service 缺少角色只读契约")
	}
	if deps.Content == nil {
		return nil, fmt.Errorf("contest service 缺少 content 读取契约")
	}
	if deps.ContentImport == nil {
		return nil, fmt.Errorf("contest service 缺少 content 导入契约")
	}
	if deps.Sandbox == nil {
		return nil, fmt.Errorf("contest service 缺少 sandbox 契约")
	}
	if deps.Judge == nil {
		return nil, fmt.Errorf("contest service 缺少 judge 契约")
	}
	if deps.Fingerprint == nil {
		return nil, fmt.Errorf("contest service 缺少 fingerprint 契约")
	}
	if deps.Notify == nil {
		return nil, fmt.Errorf("contest service 缺少 notify 契约")
	}
	if deps.Bus == nil {
		return nil, fmt.Errorf("contest service 缺少事件总线")
	}
	if deps.Cipher == nil {
		return nil, fmt.Errorf("contest service 缺少配置加密器")
	}
	if deps.Config.VulnSourceMaxResponseBytes <= 0 || deps.Config.VulnSourceTimeoutSeconds <= 0 || deps.Config.MatchmakerPollIntervalSeconds <= 0 || deps.Config.AutoArchivePollIntervalSeconds <= 0 || deps.Config.BattleSandboxReadyTimeoutSeconds <= 0 || deps.Config.BattleSandboxReadyPollIntervalMs <= 0 || deps.Config.MatchmakerBatchSize <= 0 || deps.Config.SubmitRateLimitSeconds <= 0 || deps.Config.FailedCooldownSeconds <= 0 || deps.Config.BattleELOInitialScore <= 0 || deps.Config.BattleELOKFactor <= 0 {
		return nil, fmt.Errorf("contest service 配置不完整")
	}
	return &Service{store: deps.Store, ids: deps.IDs, cfg: deps.Config, audit: deps.Audit, roles: deps.Roles, content: deps.Content, contentImport: deps.ContentImport, sandbox: deps.Sandbox, judge: deps.Judge, fingerprint: deps.Fingerprint, notify: deps.Notify, bus: deps.Bus, cipher: deps.Cipher}, nil
}

// currentIdentity 读取租户账号身份。
func currentIdentity(ctx context.Context) (tenant.Identity, error) {
	id, ok := tenant.FromContext(ctx)
	if !ok || id.TenantID <= 0 || id.AccountID <= 0 {
		return tenant.Identity{}, apperr.ErrUnauthorized
	}
	return id, nil
}

// currentServiceTenant 读取内部服务租户边界。
func currentServiceTenant(ctx context.Context) (int64, error) {
	id, ok := tenant.FromContext(ctx)
	if !ok || id.TenantID <= 0 || !id.IsSystem {
		return 0, apperr.ErrServiceUnauthorized
	}
	return id.TenantID, nil
}

// isSchoolAdmin 判断当前账号是否具备学校管理员权限。
func (s *Service) isSchoolAdmin(ctx context.Context, accountID int64) bool {
	ok, err := s.roles.HasRole(ctx, accountID, contracts.RoleSchoolAdmin)
	return err == nil && ok
}

// loadContestForManage 读取竞赛并校验教师或学校管理员管理权限。
func (s *Service) loadContestForManage(ctx context.Context, tx TxStore, tenantID, accountID, contestID int64) (Contest, error) {
	item, err := tx.GetContest(ctx, tenantID, contestID)
	if err != nil {
		return Contest{}, err
	}
	if err := canManageContest(accountID, s.isSchoolAdmin(ctx, accountID), item); err != nil {
		return Contest{}, err
	}
	return item, nil
}
