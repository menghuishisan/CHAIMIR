// contracts 定义跨模块身份角色的字符串编码、数据库枚举与双向映射。
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
	// RolePlatformAdmin 表示 SaaS 平台管理员角色。
	RolePlatformAdmin = "platform_admin"
	// RoleSchoolAdmin 表示租户内学校管理员角色。
	RoleSchoolAdmin = "school_admin"
	// RoleTeacher 表示租户内教师角色。
	RoleTeacher = "teacher"
	// RoleStudent 表示租户内学生角色。
	RoleStudent = "student"
)

// RoleCode 把数据库角色枚举转成跨模块稳定字符串。
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

// RoleNumber 把跨模块稳定字符串转成数据库角色枚举。
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
