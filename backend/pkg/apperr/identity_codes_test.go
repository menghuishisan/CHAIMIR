// apperr identity_codes_test 文件守护 M1 身份与租户错误码的一错一码规范。
package apperr

import "testing"

// TestIdentityErrorsUseDedicatedSegments 验证身份模块错误不复用通用错误码。
func TestIdentityErrorsUseDedicatedSegments(t *testing.T) {
	tests := []*Error{
		ErrIdentityInvalidPhone,
		ErrIdentityWeakPassword,
		ErrIdentityInvalidTenantCode,
		ErrIdentityInvalidCredentials,
		ErrIdentityAccountDisabled,
		ErrIdentityAccountLocked,
		ErrIdentityTenantDisabled,
		ErrIdentityTenantExpired,
		ErrIdentityActivationInvalid,
		ErrIdentitySMSNeedsTenant,
		ErrIdentitySMSTooFrequent,
		ErrIdentitySMSDailyLimited,
		ErrIdentitySMSInvalid,
		ErrIdentitySMSAttemptsLimited,
		ErrIdentitySSOServiceOriginDenied,
		ErrIdentitySSOInsecureConfig,
		ErrIdentitySSOMatchFieldInvalid,
		ErrIdentitySSOCASServerInsecure,
		ErrIdentityLDAPServerInsecure,
		ErrIdentitySSOTypeInvalid,
		ErrIdentitySSOTicketInvalid,
		ErrIdentitySSOAccountNotMatched,
		ErrIdentitySSOResponseInvalid,
		ErrIdentitySSOSecretInvalid,
		ErrIdentityTenantStatusInvalid,
		ErrIdentityTenantConfigInvalid,
		ErrIdentityOrgInvalidInput,
		ErrIdentityPlatformLayerDisabled,
		ErrIdentityBootstrapInvalid,
		ErrIdentityImportTypeInvalid,
		ErrIdentityImportUnsupportedFile,
		ErrIdentityImportContentInvalid,
		ErrIdentityImportPreviewExpired,
		ErrIdentityImportCSVFormatInvalid,
		ErrIdentityImportEmpty,
		ErrIdentityImportFormatInvalid,
		ErrIdentityImportFileTooLarge,
		ErrIdentityTeacherAdminRequired,
		ErrIdentityBaseRoleInvalid,
		ErrIdentitySessionContextMissing,
		ErrIdentityAccountBatchEmpty,
		ErrIdentityAccountBatchInvalid,
		ErrIdentityAccountUpdateInvalid,
		ErrIdentityPhoneAlreadyUsed,
	}

	seen := map[string]bool{}
	for _, errTemplate := range tests {
		if errTemplate == nil {
			t.Fatalf("identity error template must not be nil")
		}
		if errTemplate.Code == CodeBadRequest || errTemplate.Code == CodeUnauthorized || errTemplate.Code == CodeForbidden || errTemplate.Code == CodeConflict || errTemplate.Code == CodeRateLimited {
			t.Fatalf("identity error %q reused generic code %s", errTemplate.Message, errTemplate.Code)
		}
		if errTemplate.Code[0:2] != "12" && errTemplate.Code[0:2] != "13" && errTemplate.Code[0:2] != "14" {
			t.Fatalf("identity error %q used code %s outside M1 segments", errTemplate.Message, errTemplate.Code)
		}
		if seen[errTemplate.Code] {
			t.Fatalf("duplicate identity error code %s", errTemplate.Code)
		}
		seen[errTemplate.Code] = true
	}
}
