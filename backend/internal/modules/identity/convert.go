// identity 转换文件只做领域模型、DTO 和契约对象之间的纯映射。
package identity

import (
	"chaimir/internal/contracts"
	"chaimir/internal/platform/ids"
	"chaimir/internal/platform/jsonx"
	"chaimir/internal/platform/secretmap"
	"chaimir/internal/platform/timex"
	"chaimir/pkg/apperr"
	"chaimir/pkg/privacy"
)

// ToTenantDTO 把租户领域快照转换为 HTTP 响应 DTO。
func ToTenantDTO(t Tenant) TenantDTO {
	return TenantDTO{
		ID:                   ids.ID(t.ID),
		Code:                 t.Code,
		Name:                 t.Name,
		Type:                 t.Type,
		Status:               t.Status,
		DeployMode:           t.DeployMode,
		ExpireAt:             t.ExpireAt,
		LogoURL:              t.LogoURL,
		DisplayName:          t.DisplayName,
		AuthMode:             t.AuthMode,
		EnableActivationCode: t.EnableActivationCode,
	}
}

// ToAccountDTO 把账号领域快照转换为 HTTP 响应 DTO。
func ToAccountDTO(a Account, phonePlain string) AccountDTO {
	dto := AccountDTO{
		ID:           ids.ID(a.ID),
		TenantID:     ids.ID(a.TenantID),
		Name:         a.Name,
		No:           a.No,
		BaseIdentity: a.BaseIdentity,
		Roles:        a.Roles,
		Status:       a.Status,
		Title:        a.Title,
	}
	if !a.CreatedAt.IsZero() {
		dto.CreatedAt = a.CreatedAt.Format("2006-01-02T15:04:05Z07:00")
	}
	if phonePlain != "" {
		dto.PhoneMasked = privacy.MaskPhone(phonePlain)
	}
	return dto
}

// ToSessionDTO 把服务端会话快照转换为个人中心响应,不暴露 Refresh 哈希。
func ToSessionDTO(session AuthSession) SessionDTO {
	return SessionDTO{
		ID:         ids.ID(session.ID),
		DeviceInfo: session.DeviceInfo,
		IP:         session.IP,
		Status:     session.Status,
		ExpireAt:   session.ExpireAt.Format("2006-01-02T15:04:05Z07:00"),
		CreatedAt:  session.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
	}
}

// ToPlatformSessionDTO 把平台管理员会话转换为个人中心响应,不暴露 Refresh 哈希。
func ToPlatformSessionDTO(session PlatformAuthSession) SessionDTO {
	return SessionDTO{
		ID:         ids.ID(session.ID),
		DeviceInfo: session.DeviceInfo,
		IP:         session.IP,
		Status:     session.Status,
		ExpireAt:   session.ExpireAt.Format("2006-01-02T15:04:05Z07:00"),
		CreatedAt:  session.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
	}
}

// ToAuditLogDTO 把审计契约视图转换为 HTTP 响应 DTO。
func ToAuditLogDTO(row contracts.AuditLogEntry) AuditLogDTO {
	return AuditLogDTO{
		ID:         ids.ID(row.ID),
		TenantID:   ids.ID(row.TenantID),
		ActorID:    ids.ID(row.ActorID),
		ActorRole:  row.ActorRole,
		Action:     row.Action,
		TargetType: row.TargetType,
		TargetID:   ids.ID(row.TargetID),
		Detail:     row.Detail,
		IP:         row.IP,
		TraceID:    row.TraceID,
		CreatedAt:  row.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
	}
}

// ToTenantApplicationDTO 将入驻申请领域模型转换为对外 DTO。
func ToTenantApplicationDTO(app TenantApplication) TenantApplicationDTO {
	reviewedAt := ""
	if app.Status == ApplicationStatusApproved || app.Status == ApplicationStatusRejected || app.ReviewedBy > 0 {
		reviewedAt = timex.RFC3339OrEmpty(app.UpdatedAt)
	}
	return TenantApplicationDTO{
		ApplicationID: ids.ID(app.ID),
		SchoolName:    app.SchoolName,
		SchoolType:    app.SchoolType,
		ContactName:   app.ContactName,
		ContactPhone:  app.ContactPhone,
		ContactEmail:  app.ContactEmail,
		Status:        app.Status,
		RejectReason:  app.RejectReason,
		ReviewedBy:    ids.ID(app.ReviewedBy),
		TenantID:      ids.ID(app.TenantID),
		SubmittedAt:   timex.RFC3339OrEmpty(app.CreatedAt),
		ReviewedAt:    reviewedAt,
	}
}

// ToTenantApplicationDTOs 批量转换入驻申请并保持原有顺序。
func ToTenantApplicationDTOs(apps []TenantApplication) []TenantApplicationDTO {
	out := make([]TenantApplicationDTO, 0, len(apps))
	for _, app := range apps {
		out = append(out, ToTenantApplicationDTO(app))
	}
	return out
}

// ToContractAccount 把账号领域快照转换为跨模块最小账号摘要。
func ToContractAccount(a Account, phonePlain string) contracts.AccountInfo {
	roles := make([]string, 0, len(a.Roles))
	for _, role := range a.Roles {
		roles = append(roles, contracts.RoleCode(role))
	}
	return contracts.AccountInfo{
		AccountID:    a.ID,
		TenantID:     a.TenantID,
		Name:         a.Name,
		PhoneMasked:  privacy.MaskPhone(phonePlain),
		No:           a.No,
		BaseIdentity: a.BaseIdentity,
		Roles:        roles,
		Status:       a.Status,
	}
}

// ToSSOConfigDTO 把 SSO 配置转换为 HTTP DTO,返回前复用基础层递归脱敏凭据字段。
func ToSSOConfigDTO(cfg SSOConfig) (SSOConfigDTO, error) {
	data, err := jsonx.ObjectMapStrict(cfg.Config)
	if err != nil {
		return SSOConfigDTO{}, apperr.ErrIdentitySSOConfigInvalid.WithCause(err)
	}
	return SSOConfigDTO{
		ID:         ids.ID(cfg.ID),
		TenantID:   ids.ID(cfg.TenantID),
		Type:       cfg.Type,
		Config:     secretmap.Mask(data),
		MatchField: cfg.MatchField,
		Enabled:    cfg.Enabled,
	}, nil
}

// ToDepartmentDTO 把院系领域快照转换为 HTTP DTO。
func ToDepartmentDTO(dep Department) DepartmentDTO {
	return DepartmentDTO{ID: ids.ID(dep.ID), TenantID: ids.ID(dep.TenantID), Name: dep.Name, Code: dep.Code}
}

// ToMajorDTO 把专业领域快照转换为 HTTP DTO。
func ToMajorDTO(major Major) MajorDTO {
	return MajorDTO{ID: ids.ID(major.ID), TenantID: ids.ID(major.TenantID), DepartmentID: ids.ID(major.DepartmentID), Name: major.Name}
}

// ToClassDTO 把班级领域快照转换为 HTTP DTO。
func ToClassDTO(class Class) ClassDTO {
	return ClassDTO{ID: ids.ID(class.ID), TenantID: ids.ID(class.TenantID), MajorID: ids.ID(class.MajorID), Name: class.Name, EnrollmentYear: class.EnrollmentYear, Status: class.Status}
}

// ToImportBatchDTO 把导入批次领域快照转换为稳定 HTTP DTO,避免响应暴露 Go 内部字段名。
func ToImportBatchDTO(batch ImportBatch) ImportBatchDTO {
	return ImportBatchDTO{
		ID:         ids.ID(batch.ID),
		TenantID:   ids.ID(batch.TenantID),
		OperatorID: ids.ID(batch.OperatorID),
		TargetType: batch.TargetType,
		FileName:   batch.FileName,
		Total:      batch.Total,
		Success:    batch.Success,
		Failed:     batch.Failed,
		Status:     batch.Status,
		CreatedAt:  batch.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
	}
}
