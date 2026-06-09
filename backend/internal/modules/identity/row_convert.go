// identity 行转换文件只负责 sqlc 行类型到模块领域模型的纯映射。
package identity

import (
	"chaimir/internal/modules/identity/internal/sqlcgen"
	"chaimir/internal/platform/pgtypex"
	"chaimir/internal/platform/timex"
)

// tenantFromRow 转换租户 sqlc 行为领域模型。
func tenantFromRow(row sqlcgen.Tenant) Tenant {
	return Tenant{
		ID:                   row.ID,
		Code:                 row.Code,
		Name:                 row.Name,
		Type:                 row.Type,
		Status:               row.Status,
		DeployMode:           row.DeployMode,
		ExpireAt:             timex.PtrFromTimestamptz(row.ExpireAt),
		LogoURL:              pgtypex.TextValue(row.LogoUrl),
		DisplayName:          pgtypex.TextValue(row.DisplayName),
		FeatureFlags:         row.FeatureFlags,
		AuthMode:             row.AuthMode,
		EnableActivationCode: row.EnableActivationCode,
		CreatedAt:            timex.FromTimestamptz(row.CreatedAt),
		UpdatedAt:            timex.FromTimestamptz(row.UpdatedAt),
	}
}

// applicationFromRow 转换入驻申请 sqlc 行为领域模型。
func applicationFromRow(row sqlcgen.TenantApplication) TenantApplication {
	return TenantApplication{
		ID:           row.ID,
		SchoolName:   row.SchoolName,
		SchoolType:   row.SchoolType,
		ContactName:  row.ContactName,
		ContactPhone: row.ContactPhone,
		ContactEmail: row.ContactEmail,
		Status:       row.Status,
		RejectReason: pgtypex.TextValue(row.RejectReason),
		ReviewedBy:   pgtypex.Int8Value(row.ReviewedBy),
		TenantID:     pgtypex.Int8Value(row.TenantID),
		CreatedAt:    timex.FromTimestamptz(row.CreatedAt),
		UpdatedAt:    timex.FromTimestamptz(row.UpdatedAt),
	}
}

// accountFromRow 转换单账号查询行为领域模型。
func accountFromRow(row sqlcgen.GetAccountByIDRow) Account {
	return Account{
		ID:             row.ID,
		TenantID:       row.TenantID,
		PhoneEnc:       row.PhoneEnc,
		PhoneHash:      row.PhoneHash,
		PasswordHash:   pgtypex.TextValue(row.PasswordHash),
		Name:           row.Name,
		BaseIdentity:   row.BaseIdentity,
		Status:         row.Status,
		MustChangePwd:  row.MustChangePwd,
		PwdFailedCount: row.PwdFailedCount,
		LockedUntil:    timex.PtrFromTimestamptz(row.LockedUntil),
		ActivatedAt:    timex.PtrFromTimestamptz(row.ActivatedAt),
		No:             pgtypex.TextValue(row.No),
		OrgID:          pgtypex.Int8Value(row.OrgID),
		EnrollmentYear: pgtypex.Int2Value(row.EnrollmentYear),
		Title:          pgtypex.TextValue(row.Title),
		Roles:          row.Roles,
		CreatedAt:      timex.FromTimestamptz(row.CreatedAt),
		UpdatedAt:      timex.FromTimestamptz(row.UpdatedAt),
	}
}

// accountFromBatchRow 转换批量账号查询行为领域模型。
func accountFromBatchRow(row sqlcgen.BatchGetAccountsRow) Account {
	return Account{
		ID:             row.ID,
		TenantID:       row.TenantID,
		PhoneEnc:       row.PhoneEnc,
		PhoneHash:      row.PhoneHash,
		PasswordHash:   pgtypex.TextValue(row.PasswordHash),
		Name:           row.Name,
		BaseIdentity:   row.BaseIdentity,
		Status:         row.Status,
		MustChangePwd:  row.MustChangePwd,
		PwdFailedCount: row.PwdFailedCount,
		LockedUntil:    timex.PtrFromTimestamptz(row.LockedUntil),
		ActivatedAt:    timex.PtrFromTimestamptz(row.ActivatedAt),
		No:             pgtypex.TextValue(row.No),
		OrgID:          pgtypex.Int8Value(row.OrgID),
		EnrollmentYear: pgtypex.Int2Value(row.EnrollmentYear),
		Title:          pgtypex.TextValue(row.Title),
		Roles:          row.Roles,
		CreatedAt:      timex.FromTimestamptz(row.CreatedAt),
		UpdatedAt:      timex.FromTimestamptz(row.UpdatedAt),
	}
}

// authSessionFromRow 转换租户会话行为领域模型,不暴露 Refresh 明文。
func authSessionFromRow(row sqlcgen.AuthSession) AuthSession {
	return AuthSession{
		ID:               row.ID,
		TenantID:         row.TenantID,
		AccountID:        row.AccountID,
		RefreshTokenHash: row.RefreshTokenHash,
		DeviceInfo:       pgtypex.TextValue(row.DeviceInfo),
		IP:               pgtypex.TextValue(row.Ip),
		Status:           row.Status,
		ExpireAt:         timex.FromTimestamptz(row.ExpireAt),
		CreatedAt:        timex.FromTimestamptz(row.CreatedAt),
	}
}

// platformAdminFromRow 转换平台管理员行为领域模型。
func platformAdminFromRow(row sqlcgen.PlatformAdmin) PlatformAdmin {
	return PlatformAdmin{
		ID:           row.ID,
		Username:     row.Username,
		PasswordHash: row.PasswordHash,
		Name:         row.Name,
		Status:       row.Status,
	}
}

// departmentFromRow 转换院系 sqlc 行为领域模型。
func departmentFromRow(row sqlcgen.Department) Department {
	return Department{ID: row.ID, TenantID: row.TenantID, Name: row.Name, Code: pgtypex.TextValue(row.Code)}
}

// majorFromRow 转换专业 sqlc 行为领域模型。
func majorFromRow(row sqlcgen.Major) Major {
	return Major{ID: row.ID, TenantID: row.TenantID, DepartmentID: row.DepartmentID, Name: row.Name}
}

// classFromRow 转换班级 sqlc 行为领域模型。
func classFromRow(row sqlcgen.Class) Class {
	return Class{ID: row.ID, TenantID: row.TenantID, MajorID: row.MajorID, Name: row.Name, EnrollmentYear: row.EnrollmentYear, Status: row.Status}
}
