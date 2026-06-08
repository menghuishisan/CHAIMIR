// 第0层身份角色契约:定义跨模块授权时使用的稳定角色码和匹配规则。
package contracts

const (
	// RoleNumPlatformAdmin 表示 platform_admin 的数据库/审计枚举值。
	RoleNumPlatformAdmin int16 = 1
	// RoleNumSchoolAdmin 表示 school_admin 的数据库/审计枚举值。
	RoleNumSchoolAdmin int16 = 2
	// RoleNumTeacher 表示 teacher 的数据库/审计枚举值。
	RoleNumTeacher int16 = 3
	// RoleNumStudent 表示 student 的数据库/审计枚举值。
	RoleNumStudent int16 = 4
)

const (
	// RolePlatformAdmin 表示 SaaS 平台管理员角色,仅用于平台级资源与审计范围。
	RolePlatformAdmin = "platform_admin"
	// RoleStudent 表示租户内学生角色,用于学生侧学习、提交、评价等资源边界。
	RoleStudent = "student"
	// RoleTeacher 表示租户内教师角色,可管理教学、内容、实验、竞赛等教师侧资源。
	RoleTeacher = "teacher"
	// RoleSchoolAdmin 表示租户内学校管理员角色,具备学校范围内的管理授权。
	RoleSchoolAdmin = "school_admin"
)

// RoleCode 把数据库/审计角色枚举转为跨模块字符串编码。
func RoleCode(role int16) string {
	switch role {
	case RoleNumPlatformAdmin:
		return RolePlatformAdmin
	case RoleNumSchoolAdmin:
		return RoleSchoolAdmin
	case RoleNumTeacher:
		return RoleTeacher
	case RoleNumStudent:
		return RoleStudent
	default:
		return "unknown"
	}
}

// RoleNumber 把跨模块字符串角色编码转为数据库/审计角色枚举。
func RoleNumber(role string) (int16, bool) {
	switch role {
	case RolePlatformAdmin:
		return RoleNumPlatformAdmin, true
	case RoleSchoolAdmin:
		return RoleNumSchoolAdmin, true
	case RoleTeacher:
		return RoleNumTeacher, true
	case RoleStudent:
		return RoleNumStudent, true
	default:
		return 0, false
	}
}

// HasAnyRole 判断账号角色列表是否包含任一允许角色。
func HasAnyRole(actual []string, allowed ...string) bool {
	for _, role := range actual {
		for _, want := range allowed {
			if role == want {
				return true
			}
		}
	}
	return false
}
