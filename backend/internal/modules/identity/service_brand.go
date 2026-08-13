// identity service_brand 文件实现校徽上传与学校品牌读取。
//
// 校徽为什么不走统一文件服务的短时投放授权:授权在签发和消费两端都绑定登录账号
// (storage.BuildDownloadGrant 要求 account_id,下载入口再比对会话账号),
// 而校徽最主要的展示位就是**还没有登录**的私有化部署登录页 —— 那里根本不存在账号可绑。
// 为一张登录页图片开第二条免鉴权的二进制投放路由,等于绕开「所有授权只由统一入口消费」
// (docs/总-API接口总览.md §统一文件服务),因此这里把校徽当页面内容处理:
// 服务端读出对象、按内容魔数判定 MIME、以 data URI 随品牌 JSON 一起下发。
// 生产安全头的 img-src 本就允许 data:,不需要为此放宽 CSP;
// 上限由 UPLOAD_TENANT_LOGO_MAX_BYTES 控制,所以内联体积是可预期的。
//
// 课程封面不同:它只对课程内成员可见、读者一定已登录,故仍走投放授权。
// 同一平台里两种载体不是「这里一套那里一套」,而是两类资源的可见性本来不同。
//
// 上传即生效(同一请求内落 tenant.logo_ref),不做「先传对象、后由表单提交引用」两步:
// 学校管理员上传校徽时租户一定已存在(租户由入驻审核或私有化初始化创建,平台方不代传品牌资产),
// 所以没有「对象先于记录」的时序问题;而两步提交一旦被放弃,对象就成了没有引用者的垃圾。
package identity

import (
	"bytes"
	"context"
	"encoding/base64"
	"strings"

	"chaimir/internal/contracts"
	"chaimir/internal/platform/storage"
	"chaimir/internal/platform/upload"
	"chaimir/pkg/apperr"
	"chaimir/pkg/logging"
)

// TenantLogoUploadRequest 描述已读入内存的校徽文件。
type TenantLogoUploadRequest struct {
	FileName    string
	ContentType string
	Content     []byte
}

// UploadTenantLogo 校验校徽、写入对象并在同一请求内更新租户配置,返回更新后的租户视图。
func (s *Service) UploadTenantLogo(ctx context.Context, req TenantLogoUploadRequest) (TenantDTO, error) {
	id, err := requireTenantRole(ctx, s, contracts.RoleSchoolAdmin)
	if err != nil {
		return TenantDTO{}, err
	}
	if !upload.BrandImageKindValid(req.FileName, req.ContentType, req.Content) {
		return TenantDTO{}, apperr.ErrIdentityTenantLogoInvalid
	}
	if upload.CheckSize(int64(len(req.Content)), s.uploadCfg.TenantLogoMaxBytes) != upload.SizeOK {
		return TenantDTO{}, apperr.ErrIdentityTenantLogoTooLarge
	}
	plan, err := s.files.PlanUpload(ctx, storage.PlanUploadRequest{
		TenantID:        id.TenantID,
		AccountID:       id.AccountID,
		Module:          identityModuleName,
		ResourceType:    tenantLogoResourceType,
		ResourceID:      tenantLogoResourceID,
		FileName:        req.FileName,
		ContentType:     req.ContentType,
		Size:            int64(len(req.Content)),
		MaxBytes:        s.uploadCfg.TenantLogoMaxBytes,
		ExpectedBucket:  s.objects.BucketAttach(),
		AllowedFileName: true,
		Content:         req.Content,
		KindValidator:   upload.BrandImageKindValid,
		ScanPolicy:      upload.ScanPolicy{Required: s.uploadCfg.VirusScanRequired},
	})
	if err != nil {
		return TenantDTO{}, apperr.ErrIdentityTenantLogoInvalid.WithCause(err)
	}
	if err := s.objects.Put(ctx, plan.Bucket, plan.Key, bytes.NewReader(req.Content), plan.Size, plan.ContentType); err != nil {
		return TenantDTO{}, apperr.ErrIdentityTenantLogoUnavailable.WithCause(err)
	}

	var row Tenant
	var previousLogoRef string
	if err := s.store.TenantTx(ctx, id.TenantID, func(ctx context.Context, tx TxStore) error {
		current, err := tx.GetTenantByID(ctx, id.TenantID)
		if err != nil {
			return err
		}
		previousLogoRef = current.LogoRef
		// 只改校徽引用,其余配置字段原样写回:本接口不是配置更新入口。
		item, err := tx.UpdateTenantConfig(ctx, UpdateTenantConfigInput{
			TenantID:             id.TenantID,
			LogoRef:              plan.ObjectRef,
			DisplayName:          current.DisplayName,
			FeatureFlags:         current.FeatureFlags,
			AuthMode:             current.AuthMode,
			EnableActivationCode: current.EnableActivationCode,
		})
		if err != nil {
			return err
		}
		row = item
		return nil
	}); err != nil {
		// 引用没写成功时对象就是垃圾,立即清掉,不留给后台任务去猜。
		if cleanupErr := s.objects.Delete(ctx, plan.Bucket, plan.Key); cleanupErr != nil {
			logging.ErrorContext(ctx, "清理未关联校徽对象失败", cleanupErr.Error())
		}
		return TenantDTO{}, apperr.ErrInternal.WithCause(err)
	}
	// 换徽后旧对象没有任何引用者,留着只会在附件桶里堆积。
	s.discardReplacedTenantLogo(ctx, id.TenantID, previousLogoRef, row.LogoRef)
	if err := s.auditTenantOperation(ctx, id, "tenant.logo.upload", "identity.tenant", id.TenantID, map[string]any{"file_name": plan.FileName, "size": plan.Size}); err != nil {
		return TenantDTO{}, err
	}
	return s.tenantDTOWithLogo(ctx, id.TenantID, row), nil
}

// ClearTenantLogo 移除校徽:清空引用并删除对象,让徽记位回落学校名首字。
func (s *Service) ClearTenantLogo(ctx context.Context) (TenantDTO, error) {
	id, err := requireTenantRole(ctx, s, contracts.RoleSchoolAdmin)
	if err != nil {
		return TenantDTO{}, err
	}
	var row Tenant
	var previousLogoRef string
	if err := s.store.TenantTx(ctx, id.TenantID, func(ctx context.Context, tx TxStore) error {
		current, err := tx.GetTenantByID(ctx, id.TenantID)
		if err != nil {
			return err
		}
		previousLogoRef = current.LogoRef
		item, err := tx.UpdateTenantConfig(ctx, UpdateTenantConfigInput{
			TenantID:             id.TenantID,
			LogoRef:              "",
			DisplayName:          current.DisplayName,
			FeatureFlags:         current.FeatureFlags,
			AuthMode:             current.AuthMode,
			EnableActivationCode: current.EnableActivationCode,
		})
		if err != nil {
			return err
		}
		row = item
		return nil
	}); err != nil {
		return TenantDTO{}, apperr.ErrInternal.WithCause(err)
	}
	s.discardReplacedTenantLogo(ctx, id.TenantID, previousLogoRef, "")
	if err := s.auditTenantOperation(ctx, id, "tenant.logo.clear", "identity.tenant", id.TenantID, map[string]any{}); err != nil {
		return TenantDTO{}, err
	}
	return ToTenantDTO(row), nil
}

// tenantDTOWithLogo 把租户行转成配置视图并补上内联校徽,读不出校徽时只记日志不影响其余字段。
func (s *Service) tenantDTOWithLogo(ctx context.Context, tenantID int64, row Tenant) TenantDTO {
	out := ToTenantDTO(row)
	image, err := s.readTenantLogoImage(ctx, tenantID, row.LogoRef)
	if err != nil {
		logging.ErrorContext(ctx, "读取学校校徽失败", err.Error())
		return out
	}
	out.LogoImage = image
	return out
}

// TenantBrandDTO 是登录页面读取的学校品牌信息。
type TenantBrandDTO struct {
	DisplayName string `json:"display_name"`
	// LogoImage 是 data URI 形式的校徽;没有校徽或非私有化部署时为空串。
	LogoImage string `json:"logo_image"`
}

// GetTenantBrand 供登录页免鉴权读取学校品牌,不接受任何租户标识参数。
// SaaS 登录页面对的是尚未确定的租户,本就没有校徽可显示;而任何带租户标识的公开端点
// 都会成为廉价的租户枚举通道,所以这里只在私有化部署下按配置里的固定租户返回内容。
func (s *Service) GetTenantBrand(ctx context.Context) (TenantBrandDTO, error) {
	if !s.deploy.IsSchool() {
		return TenantBrandDTO{}, nil
	}
	tenantID := s.deploy.SchoolTenantID
	var row Tenant
	// 走租户连接而不是平台连接:租户 ID 来自部署配置,不需要绕过 RLS。
	// 这是个免鉴权入口,能用权限更小的连接就不该碰绕过 RLS 的那条。
	if err := s.store.TenantTx(ctx, tenantID, func(ctx context.Context, tx TxStore) error {
		item, err := tx.GetTenantByID(ctx, tenantID)
		if err != nil {
			return err
		}
		row = item
		return nil
	}); err != nil {
		return TenantBrandDTO{}, apperr.ErrInternal.WithCause(err)
	}
	name := strings.TrimSpace(row.DisplayName)
	if name == "" {
		name = row.Name
	}
	image, err := s.readTenantLogoImage(ctx, tenantID, row.LogoRef)
	if err != nil {
		// 校徽读不出来不该让登录页整体失败:名称已经够登录页表达身份,徽记位回落首字块。
		logging.ErrorContext(ctx, "读取学校校徽失败", err.Error())
		return TenantBrandDTO{DisplayName: name}, nil
	}
	return TenantBrandDTO{DisplayName: name, LogoImage: image}, nil
}

// readTenantLogoImage 把已登记的校徽对象读成 data URI;没有校徽时返回空串。
func (s *Service) readTenantLogoImage(ctx context.Context, tenantID int64, logoRef string) (string, error) {
	logoRef = strings.TrimSpace(logoRef)
	if logoRef == "" {
		return "", nil
	}
	ref, err := s.tenantLogoObject(tenantID, logoRef)
	if err != nil {
		return "", err
	}
	reader, err := s.objects.Get(ctx, ref.Bucket, ref.Key)
	if err != nil {
		return "", err
	}
	defer logging.CloseContext(ctx, "关闭校徽对象读取失败", reader)
	content, result, err := upload.ReadBounded(reader, s.uploadCfg.TenantLogoMaxBytes)
	if err != nil {
		return "", err
	}
	if result != upload.SizeOK {
		return "", apperr.ErrIdentityTenantLogoTooLarge
	}
	// MIME 由内容魔数决定:data URI 里的类型直接决定浏览器怎么解释这段字节,
	// 不能沿用上传时客户端声明的值。
	contentType := upload.BrandImageContentType(content)
	if contentType == "" {
		return "", apperr.ErrIdentityTenantLogoInvalid
	}
	return "data:" + contentType + ";base64," + base64.StdEncoding.EncodeToString(content), nil
}

// tenantLogoObject 校验校徽引用落在本租户受控前缀内,阻断把配置指向任意对象读取。
func (s *Service) tenantLogoObject(tenantID int64, logoRef string) (storage.ObjectRef, error) {
	ref, err := storage.ParseObjectRef(logoRef)
	if err != nil {
		return storage.ObjectRef{}, err
	}
	prefix, err := storage.ObjectKey(tenantID, identityModuleName, tenantLogoResourceType, tenantLogoResourceID)
	if err != nil {
		return storage.ObjectRef{}, err
	}
	if ref.Bucket != s.objects.BucketAttach() || !strings.HasPrefix(ref.Key, prefix+"/") {
		return storage.ObjectRef{}, apperr.ErrIdentityTenantLogoInvalid
	}
	return ref, nil
}

// discardReplacedTenantLogo 删除被换掉的旧校徽对象,失败只记日志不影响配置保存结果。
func (s *Service) discardReplacedTenantLogo(ctx context.Context, tenantID int64, previous, current string) {
	previous = strings.TrimSpace(previous)
	if previous == "" || previous == strings.TrimSpace(current) {
		return
	}
	ref, err := s.tenantLogoObject(tenantID, previous)
	if err != nil {
		logging.ErrorContext(ctx, "替换校徽时旧对象引用不在受控前缀", err.Error())
		return
	}
	if err := s.objects.Delete(ctx, ref.Bucket, ref.Key); err != nil {
		logging.ErrorContext(ctx, "清理已替换校徽失败", err.Error())
	}
}
