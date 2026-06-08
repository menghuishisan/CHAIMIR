// 账号开通凭据:统一处理临时密码与激活码两种开通方式。
package identity

import (
	"chaimir/pkg/crypto"

	"github.com/jackc/pgx/v5/pgtype"
)

// accountOpeningCredential 是账号创建时落库与一次性返回的开通凭据。
type accountOpeningCredential struct {
	PasswordHash        pgtype.Text
	MustChangePassword  bool
	NeedsActivationCode bool
	InitPassword        string
}

// buildAccountOpeningCredential 根据租户开通方式构造账号凭据;激活码模式不写初始密码。
func buildAccountOpeningCredential(enableActivationCode bool, initPassword string) (accountOpeningCredential, error) {
	if enableActivationCode {
		return accountOpeningCredential{NeedsActivationCode: true}, nil
	}
	hash, err := crypto.HashPassword(initPassword)
	if err != nil {
		return accountOpeningCredential{}, err
	}
	return accountOpeningCredential{
		PasswordHash:       pgText(hash),
		MustChangePassword: true,
		InitPassword:       initPassword,
	}, nil
}
