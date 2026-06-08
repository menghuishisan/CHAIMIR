// M1 账号列表查询条件解析。
package identity

import (
	"strconv"

	"chaimir/internal/platform/ids"
	"chaimir/pkg/apperr"
)

// AccountListFilter 是账号列表查询的业务过滤条件。
type AccountListFilter struct {
	Role    int16
	ClassID int64
	Status  int16
	Keyword string
}

// buildAccountListFilter 从文档定义的 role/class_id/status/keyword 查询参数构造过滤条件。
func buildAccountListFilter(roleText, classIDText, statusText, keyword string) (AccountListFilter, error) {
	var role int16
	if roleText != "" {
		n, ok := roleNumOf(roleText)
		if !ok || n == RolePlatformAdmin {
			return AccountListFilter{}, apperr.ErrAccountQueryInvalid
		}
		role = n
	}
	var classID int64
	if classIDText != "" {
		v, ok := ids.Parse(classIDText)
		if !ok {
			return AccountListFilter{}, apperr.ErrAccountQueryInvalid
		}
		classID = v
	}
	var status int16
	if statusText != "" {
		n, err := strconv.Atoi(statusText)
		if err != nil || !validAccountStatus(int16(n)) {
			return AccountListFilter{}, apperr.ErrAccountQueryInvalid
		}
		status = int16(n)
	}
	return AccountListFilter{Role: role, ClassID: classID, Status: status, Keyword: keyword}, nil
}

// validAccountStatus 判断查询状态是否属于账号状态机枚举。
func validAccountStatus(v int16) bool {
	switch v {
	case AccountPending, AccountActive, AccountDisabled, AccountArchived, AccountCancelled:
		return true
	default:
		return false
	}
}
