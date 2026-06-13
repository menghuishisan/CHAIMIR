// apperr identity_codes 文件定义 M1 身份与租户模块专属 12xxx/13xxx/14xxx 错误码。
package apperr

const (
	// CodeIdentityInvalidPhone 表示手机号格式不符合身份模块规则。
	CodeIdentityInvalidPhone = "12001"
	// CodeIdentityWeakPassword 表示密码强度不符合身份模块规则。
	CodeIdentityWeakPassword = "12002"
	// CodeIdentityInvalidTenantCode 表示学校短码格式不正确。
	CodeIdentityInvalidTenantCode = "12003"
	// CodeIdentityInvalidCredentials 表示账号或密码、验证码等认证凭据无效。
	CodeIdentityInvalidCredentials = "12004"
	// CodeIdentityAccountDisabled 表示账号当前状态不允许登录或操作。
	CodeIdentityAccountDisabled = "12005"
	// CodeIdentityAccountLocked 表示账号因失败次数过多被临时锁定。
	CodeIdentityAccountLocked = "12006"
	// CodeIdentityActivationInvalid 表示激活码无效、过期或已使用。
	CodeIdentityActivationInvalid = "12007"
	// CodeIdentitySMSNeedsTenant 表示短信场景需要先明确租户。
	CodeIdentitySMSNeedsTenant = "12008"
	// CodeIdentitySMSTooFrequent 表示短信发送触发同号短时限频。
	CodeIdentitySMSTooFrequent = "12009"
	// CodeIdentitySMSDailyLimited 表示短信发送触发每日上限。
	CodeIdentitySMSDailyLimited = "12010"
	// CodeIdentitySMSInvalid 表示短信验证码不正确、过期或已使用。
	CodeIdentitySMSInvalid = "12011"
	// CodeIdentitySMSAttemptsLimited 表示短信验证码校验次数超过上限。
	CodeIdentitySMSAttemptsLimited = "12012"
	// CodeIdentityTeacherAdminRequired 表示学校管理员权限只能授予教师账号。
	CodeIdentityTeacherAdminRequired = "12013"
	// CodeIdentityPlatformLoginDisabled 表示当前部署禁用平台管理员入口。
	CodeIdentityPlatformLoginDisabled = "12014"
	// CodeIdentitySessionInvalid 表示 Refresh 或服务端会话无效。
	CodeIdentitySessionInvalid = "12015"
	// CodeIdentityBaseRoleInvalid 表示账号基础身份类型不正确。
	CodeIdentityBaseRoleInvalid = "12016"
	// CodeIdentitySessionContextMissing 表示已鉴权请求缺少服务端会话上下文。
	CodeIdentitySessionContextMissing = "12017"
	// CodeIdentityAccountBatchEmpty 表示批量账号操作没有提交账号。
	CodeIdentityAccountBatchEmpty = "12018"
	// CodeIdentityAccountBatchInvalid 表示批量账号操作包含非法账号 ID。
	CodeIdentityAccountBatchInvalid = "12019"
	// CodeIdentityAccountUpdateInvalid 表示账号编辑字段不完整或不允许。
	CodeIdentityAccountUpdateInvalid = "12020"
	// CodeIdentityPhoneAlreadyUsed 表示换绑手机号已被本租户其他账号使用。
	CodeIdentityPhoneAlreadyUsed = "12021"
	// CodeIdentityActivationDisabled 表示租户未启用激活码开通方式。
	CodeIdentityActivationDisabled = "12022"
)

const (
	// CodeIdentityTenantDisabled 表示租户被停用。
	CodeIdentityTenantDisabled = "13001"
	// CodeIdentityTenantExpired 表示租户服务已到期。
	CodeIdentityTenantExpired = "13002"
	// CodeIdentityApplicationInvalid 表示入驻申请信息不完整或无效。
	CodeIdentityApplicationInvalid = "13003"
	// CodeIdentitySSOConfigInvalid 表示统一认证配置格式或字段不正确。
	CodeIdentitySSOConfigInvalid = "13004"
	// CodeIdentitySSOServiceOriginDenied 表示统一认证回调来源不在允许列表。
	CodeIdentitySSOServiceOriginDenied = "13005"
	// CodeIdentitySSONotEnabled 表示学校不存在或未启用对应统一认证。
	CodeIdentitySSONotEnabled = "13006"
	// CodeIdentitySSOInsecureConfig 表示统一认证端点未满足 HTTPS/LDAPS 等安全要求。
	CodeIdentitySSOInsecureConfig = "13007"
	// CodeIdentitySSOMatchFieldInvalid 表示统一认证名单匹配字段不正确。
	CodeIdentitySSOMatchFieldInvalid = "13008"
	// CodeIdentitySSOCASServerInsecure 表示 CAS 服务地址未使用安全 HTTPS 地址。
	CodeIdentitySSOCASServerInsecure = "13009"
	// CodeIdentityLDAPServerInsecure 表示 LDAP 服务地址未使用 LDAPS。
	CodeIdentityLDAPServerInsecure = "13010"
	// CodeIdentitySSOTypeInvalid 表示统一认证类型不受支持。
	CodeIdentitySSOTypeInvalid = "13011"
	// CodeIdentitySSOTicketInvalid 表示 CAS 票据校验失败或已失效。
	CodeIdentitySSOTicketInvalid = "13012"
	// CodeIdentitySSOAccountNotMatched 表示统一认证用户未命中已导入账号名单。
	CodeIdentitySSOAccountNotMatched = "13013"
	// CodeIdentitySSOResponseInvalid 表示统一认证服务返回内容不符合协议。
	CodeIdentitySSOResponseInvalid = "13014"
	// CodeIdentitySSOSecretInvalid 表示统一认证敏感配置无法安全处理。
	CodeIdentitySSOSecretInvalid = "13015"
	// CodeIdentityTenantStatusInvalid 表示平台提交的租户状态不在允许状态机内。
	CodeIdentityTenantStatusInvalid = "13016"
	// CodeIdentityTenantConfigInvalid 表示学校管理员提交的租户配置格式不正确。
	CodeIdentityTenantConfigInvalid = "13017"
	// CodeIdentityOrgInvalidInput 表示组织架构请求字段不完整或状态不正确。
	CodeIdentityOrgInvalidInput = "13018"
	// CodeIdentityPlatformLayerDisabled 表示当前部署不启用 SaaS 平台管理层。
	CodeIdentityPlatformLayerDisabled = "13019"
	// CodeIdentityBootstrapInvalid 表示私有化初始化参数不完整或不合法。
	CodeIdentityBootstrapInvalid = "13020"
	// CodeIdentityRouteDependencyMissing 表示身份模块 HTTP 路由装配缺少必要依赖。
	CodeIdentityRouteDependencyMissing = "13021"
)

const (
	// CodeIdentityImportTypeInvalid 表示导入目标类型不正确。
	CodeIdentityImportTypeInvalid = "14001"
	// CodeIdentityImportUnsupportedFile 表示导入文件类型不受支持。
	CodeIdentityImportUnsupportedFile = "14002"
	// CodeIdentityImportContentInvalid 表示导入文件内容无法解析或为空。
	CodeIdentityImportContentInvalid = "14003"
	// CodeIdentityImportTooManyRows 表示导入行数超过身份模块配置上限。
	CodeIdentityImportTooManyRows = "14004"
	// CodeIdentityImportPreviewExpired 表示导入预览已失效或已提交。
	CodeIdentityImportPreviewExpired = "14005"
	// CodeIdentityImportCSVFormatInvalid 表示 CSV 导入文件格式不正确。
	CodeIdentityImportCSVFormatInvalid = "14006"
	// CodeIdentityImportEmpty 表示导入文件没有可导入的数据。
	CodeIdentityImportEmpty = "14007"
	// CodeIdentityImportFormatInvalid 表示导入模板格式参数不受支持。
	CodeIdentityImportFormatInvalid = "14008"
	// CodeIdentityImportFileTooLarge 表示导入文件超过上传大小上限。
	CodeIdentityImportFileTooLarge = "14009"
)

var (
	// ErrIdentityInvalidPhone 表示手机号格式不正确。
	ErrIdentityInvalidPhone = New(CodeIdentityInvalidPhone, "手机号格式不正确,请检查后重试")
	// ErrIdentityWeakPassword 表示密码强度不足。
	ErrIdentityWeakPassword = New(CodeIdentityWeakPassword, "密码至少需要 8 位,并包含字母和数字")
	// ErrIdentityInvalidTenantCode 表示学校短码格式不正确。
	ErrIdentityInvalidTenantCode = New(CodeIdentityInvalidTenantCode, "学校短码格式不正确")
	// ErrIdentityInvalidCredentials 表示认证凭据无效。
	ErrIdentityInvalidCredentials = New(CodeIdentityInvalidCredentials, "账号或密码不正确")
	// ErrIdentityAccountDisabled 表示账号状态禁止登录。
	ErrIdentityAccountDisabled = New(CodeIdentityAccountDisabled, "当前账号暂时无法登录,请联系学校管理员")
	// ErrIdentityAccountLocked 表示账号被临时锁定。
	ErrIdentityAccountLocked = New(CodeIdentityAccountLocked, "登录失败次数过多,请稍后再试")
	// ErrIdentityActivationInvalid 表示激活码不可用。
	ErrIdentityActivationInvalid = New(CodeIdentityActivationInvalid, "激活码无效或已过期")
	// ErrIdentitySMSNeedsTenant 表示短信发送需要先选择学校。
	ErrIdentitySMSNeedsTenant = New(CodeIdentitySMSNeedsTenant, "请选择学校后再获取验证码")
	// ErrIdentitySMSTooFrequent 表示短信发送过于频繁。
	ErrIdentitySMSTooFrequent = New(CodeIdentitySMSTooFrequent, "验证码发送过于频繁,请稍后再试")
	// ErrIdentitySMSDailyLimited 表示短信发送达到每日上限。
	ErrIdentitySMSDailyLimited = New(CodeIdentitySMSDailyLimited, "今日验证码次数已达上限,请明天再试")
	// ErrIdentitySMSInvalid 表示短信验证码不可用。
	ErrIdentitySMSInvalid = New(CodeIdentitySMSInvalid, "验证码不正确或已过期")
	// ErrIdentitySMSAttemptsLimited 表示短信验证码尝试次数过多。
	ErrIdentitySMSAttemptsLimited = New(CodeIdentitySMSAttemptsLimited, "验证码错误次数过多,请重新获取")
	// ErrIdentityTeacherAdminRequired 表示只能向教师授予学校管理员权限。
	ErrIdentityTeacherAdminRequired = New(CodeIdentityTeacherAdminRequired, "只能授予教师学校管理员权限")
	// ErrIdentityPlatformLoginDisabled 表示平台登录入口被部署配置关闭。
	ErrIdentityPlatformLoginDisabled = New(CodeIdentityPlatformLoginDisabled, "当前部署未启用平台管理员入口")
	// ErrIdentitySessionInvalid 表示登录态或刷新会话无效。
	ErrIdentitySessionInvalid = New(CodeIdentitySessionInvalid, "登录已失效,请重新登录")
	// ErrIdentityBaseRoleInvalid 表示账号基础身份类型不正确。
	ErrIdentityBaseRoleInvalid = New(CodeIdentityBaseRoleInvalid, "账号身份类型不正确")
	// ErrIdentitySessionContextMissing 表示服务端会话上下文缺失。
	ErrIdentitySessionContextMissing = New(CodeIdentitySessionContextMissing, "登录状态暂时无法确认,请重新登录")
	// ErrIdentityAccountBatchEmpty 表示批量账号操作没有账号。
	ErrIdentityAccountBatchEmpty = New(CodeIdentityAccountBatchEmpty, "请选择要操作的账号")
	// ErrIdentityAccountBatchInvalid 表示批量账号操作包含无效账号。
	ErrIdentityAccountBatchInvalid = New(CodeIdentityAccountBatchInvalid, "账号选择不正确,请检查后重试")
	// ErrIdentityAccountUpdateInvalid 表示账号编辑请求不正确。
	ErrIdentityAccountUpdateInvalid = New(CodeIdentityAccountUpdateInvalid, "账号信息不正确,请检查后重试")
	// ErrIdentityPhoneAlreadyUsed 表示手机号已被其他账号使用。
	ErrIdentityPhoneAlreadyUsed = New(CodeIdentityPhoneAlreadyUsed, "该手机号已被使用,请更换后重试")
	// ErrIdentityActivationDisabled 表示学校未启用激活码开通。
	ErrIdentityActivationDisabled = New(CodeIdentityActivationDisabled, "学校未启用激活码开通,请填写初始密码")
)

var (
	// ErrIdentityTenantDisabled 表示学校服务停用。
	ErrIdentityTenantDisabled = New(CodeIdentityTenantDisabled, "学校服务已停用,请联系学校管理员")
	// ErrIdentityTenantExpired 表示学校服务到期。
	ErrIdentityTenantExpired = New(CodeIdentityTenantExpired, "学校服务已到期,请联系学校管理员")
	// ErrIdentityApplicationInvalid 表示入驻申请缺少必要信息。
	ErrIdentityApplicationInvalid = New(CodeIdentityApplicationInvalid, "请完整填写学校和联系人信息")
	// ErrIdentitySSOConfigInvalid 表示统一认证配置不合法。
	ErrIdentitySSOConfigInvalid = New(CodeIdentitySSOConfigInvalid, "认证配置格式不正确")
	// ErrIdentitySSOServiceOriginDenied 表示统一认证回调地址不受信任。
	ErrIdentitySSOServiceOriginDenied = New(CodeIdentitySSOServiceOriginDenied, "统一认证回调地址不受信任")
	// ErrIdentitySSONotEnabled 表示学校不存在或未启用统一认证。
	ErrIdentitySSONotEnabled = New(CodeIdentitySSONotEnabled, "学校不存在或未启用统一认证")
	// ErrIdentitySSOInsecureConfig 表示统一认证端点配置不安全。
	ErrIdentitySSOInsecureConfig = New(CodeIdentitySSOInsecureConfig, "统一认证服务配置不安全")
	// ErrIdentitySSOMatchFieldInvalid 表示统一认证名单匹配字段不正确。
	ErrIdentitySSOMatchFieldInvalid = New(CodeIdentitySSOMatchFieldInvalid, "名单匹配字段不正确")
	// ErrIdentitySSOCASServerInsecure 表示 CAS 服务地址不安全。
	ErrIdentitySSOCASServerInsecure = New(CodeIdentitySSOCASServerInsecure, "CAS 服务地址必须使用安全的 HTTPS 地址")
	// ErrIdentityLDAPServerInsecure 表示 LDAP 服务地址不安全。
	ErrIdentityLDAPServerInsecure = New(CodeIdentityLDAPServerInsecure, "LDAP 服务地址必须使用 LDAPS")
	// ErrIdentitySSOTypeInvalid 表示统一认证类型不正确。
	ErrIdentitySSOTypeInvalid = New(CodeIdentitySSOTypeInvalid, "统一认证类型不正确")
	// ErrIdentitySSOTicketInvalid 表示 CAS 票据校验失败。
	ErrIdentitySSOTicketInvalid = New(CodeIdentitySSOTicketInvalid, "统一认证票据无效或已过期")
	// ErrIdentitySSOAccountNotMatched 表示统一认证用户未在导入名单中。
	ErrIdentitySSOAccountNotMatched = New(CodeIdentitySSOAccountNotMatched, "账号未在学校名单中")
	// ErrIdentitySSOResponseInvalid 表示统一认证服务响应无法识别。
	ErrIdentitySSOResponseInvalid = New(CodeIdentitySSOResponseInvalid, "统一认证服务响应异常")
	// ErrIdentitySSOSecretInvalid 表示统一认证敏感配置无法安全处理。
	ErrIdentitySSOSecretInvalid = New(CodeIdentitySSOSecretInvalid, "统一认证敏感配置无法处理")
	// ErrIdentityTenantStatusInvalid 表示租户状态不正确。
	ErrIdentityTenantStatusInvalid = New(CodeIdentityTenantStatusInvalid, "学校状态不正确")
	// ErrIdentityTenantConfigInvalid 表示租户配置格式不正确。
	ErrIdentityTenantConfigInvalid = New(CodeIdentityTenantConfigInvalid, "学校配置格式不正确")
	// ErrIdentityOrgInvalidInput 表示组织架构请求信息不正确。
	ErrIdentityOrgInvalidInput = New(CodeIdentityOrgInvalidInput, "组织架构信息不正确,请检查后重试")
	// ErrIdentityPlatformLayerDisabled 表示当前部署不启用平台管理。
	ErrIdentityPlatformLayerDisabled = New(CodeIdentityPlatformLayerDisabled, "当前部署未启用平台管理入口")
	// ErrIdentityBootstrapInvalid 表示私有化初始化参数不正确。
	ErrIdentityBootstrapInvalid = New(CodeIdentityBootstrapInvalid, "初始化学校信息不完整,请检查配置")
	// ErrIdentityRouteDependencyMissing 表示身份模块 HTTP 路由装配缺少必要依赖。
	ErrIdentityRouteDependencyMissing = New(CodeIdentityRouteDependencyMissing, "服务暂时不可用,请稍后重试")
)

var (
	// ErrIdentityImportTypeInvalid 表示导入目标类型错误。
	ErrIdentityImportTypeInvalid = New(CodeIdentityImportTypeInvalid, "导入类型不正确")
	// ErrIdentityImportUnsupportedFile 表示导入文件类型不支持。
	ErrIdentityImportUnsupportedFile = New(CodeIdentityImportUnsupportedFile, "当前仅支持 CSV 或 Excel 导入文件")
	// ErrIdentityImportContentInvalid 表示导入文件内容错误。
	ErrIdentityImportContentInvalid = New(CodeIdentityImportContentInvalid, "导入文件内容不正确")
	// ErrIdentityImportTooManyRows 表示导入数据超过配置上限。
	ErrIdentityImportTooManyRows = New(CodeIdentityImportTooManyRows, "导入行数超过上限,请拆分后重试")
	// ErrIdentityImportPreviewExpired 表示导入预览已失效。
	ErrIdentityImportPreviewExpired = New(CodeIdentityImportPreviewExpired, "导入预览已失效,请重新上传")
	// ErrIdentityImportCSVFormatInvalid 表示 CSV 导入文件格式不正确。
	ErrIdentityImportCSVFormatInvalid = New(CodeIdentityImportCSVFormatInvalid, "导入文件格式不正确")
	// ErrIdentityImportEmpty 表示导入文件没有可导入的数据。
	ErrIdentityImportEmpty = New(CodeIdentityImportEmpty, "导入文件没有可导入的数据")
	// ErrIdentityImportFormatInvalid 表示模板格式不受支持。
	ErrIdentityImportFormatInvalid = New(CodeIdentityImportFormatInvalid, "模板格式不正确")
	// ErrIdentityImportFileTooLarge 表示导入文件过大。
	ErrIdentityImportFileTooLarge = New(CodeIdentityImportFileTooLarge, "导入文件过大,请拆分后重试")
)
