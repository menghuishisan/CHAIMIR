// Identity API：认证、授权、用户管理
// 对应后端 M1 模块

import { ApiClient } from '../client'
import type { AttachmentResponse } from '../client'
import type { AccountStatus, ApplicationStatus, ImportTemplateFormat, UserRole } from '../constants/identity'
import type { PaginatedResponse } from '../types/common'
import type {
  LoginPlatformRequest,
  LoginPhoneRequest,
  LoginNoRequest,
  LoginSMSRequest,
  SendSMSRequest,
  WebSocketTicketResponse,
  PasswordResetRequest,
  ActivateRequest,
  LoginResponse,
  Account,
  MeResponse,
  ChangePasswordRequest,
  ChangePhoneRequest,
  Session,
  CreateApplicationRequest,
  TenantApplication,
  Tenant,
  ReviewApplicationRequest,
  UpdateTenantStatusRequest,
  TenantConfigRequest,
  TenantBrand,
  SSOConfig,
  SSOConfigRequest,
  LDAPLoginRequest,
  Department,
  DepartmentRequest,
  Major,
  MajorRequest,
  Class,
  ClassRequest,
  ClassStudent,
  ArchiveClassesRequest,
  CreateAccountRequest,
  UpdateAccountRequest,
  CreateAccountResponse,
  AdminResetPasswordRequest,
  BatchAccountIDsRequest,
  ImportPreviewResponse,
  ImportCommitRequest,
  PromoteClassesRequest,
  AccountImportCommitResponse,
  ImportBatch,
} from '../types/identity'

/**
 * IdentityApi 封装后端 M1 身份、租户、组织和账号管理接口。
 */
export class IdentityApi {
  /**
   * constructor 注入统一 API 客户端，确保鉴权、trace_id 和错误信封处理一致。
   */
  constructor(private client: ApiClient) {}

  // ===== 认证 =====

  /**
   * 平台管理员登录（用户名密码）
   */
  async loginPlatform(data: LoginPlatformRequest): Promise<LoginResponse> {
    return this.client.post('/auth/login/platform', data)
  }

  /**
   * 学校用户登录（手机号密码）
   */
  async loginPhone(data: LoginPhoneRequest): Promise<LoginResponse> {
    return this.client.post('/auth/login/phone', data)
  }

  /**
   * 学校用户登录（学号密码）
   */
  async loginNo(data: LoginNoRequest): Promise<LoginResponse> {
    return this.client.post('/auth/login/no', data)
  }

  /**
   * 短信验证码登录
   */
  async loginSMS(data: LoginSMSRequest): Promise<LoginResponse> {
    return this.client.post('/auth/login/sms', data)
  }

  /**
   * 发送短信验证码
   */
  async sendSMS(data: SendSMSRequest): Promise<void> {
    return this.client.post('/auth/sms/send', data)
  }

  /**
   * 为指定实时通道换取短时连接票据。
   */
  async issueWebSocketTicket(path: string): Promise<WebSocketTicketResponse> {
    return this.client.post('/auth/ws-ticket', { path })
  }

  /**
   * 重置密码
   */
  async resetPassword(data: PasswordResetRequest): Promise<void> {
    return this.client.post('/auth/password/reset', data)
  }

  /**
   * 激活账号
   */
  async activate(data: ActivateRequest): Promise<void> {
    return this.client.post('/auth/activate', data)
  }

  /**
   * 登出
   */
  async logout(): Promise<void> {
    return this.client.post('/auth/logout')
  }

  /**
   * 获取 CAS 登录跳转地址
   */
  async getCASLoginUrl(tenantCode: string, service: string): Promise<{ redirect_url: string }> {
    return this.client.get(`/auth/sso/${tenantCode}/login`, { service })
  }

  /**
   * 处理 CAS 回调并换取登录态
   */
  async casCallback(tenantCode: string, params: { ticket: string; service: string }): Promise<LoginResponse> {
    return this.client.get(`/auth/sso/${tenantCode}/callback`, params)
  }

  /**
   * LDAP 登录
   */
  async ldapLogin(tenantCode: string, data: LDAPLoginRequest): Promise<LoginResponse> {
    return this.client.post(`/auth/sso/${tenantCode}/ldap`, data)
  }

  // ===== 当前用户 =====

  /**
   * 获取当前用户信息
   */
  async getMe(): Promise<MeResponse> {
    return this.client.get('/me')
  }

  /**
   * 修改密码
   */
  async changePassword(data: ChangePasswordRequest): Promise<void> {
    return this.client.post('/me/password', data)
  }

  /**
   * 修改手机号
   */
  async changePhone(data: ChangePhoneRequest): Promise<void> {
    return this.client.post('/me/phone', data)
  }

  /**
   * 获取当前用户会话列表
   */
  async getSessions(): Promise<Session[]> {
    return this.client.get('/me/sessions')
  }

  // ===== 账号管理（学校管理员） =====

  /**
   * 查询当前租户账号列表，供学校管理员按状态、角色、班级和关键词筛选。
   */
  async getAccounts(params?: {
    status?: AccountStatus
    role?: UserRole
    class_id?: string
    keyword?: string
    page?: number
    size?: number
  }): Promise<PaginatedResponse<Account>> {
    return this.client.get('/accounts', params)
  }

  /**
   * 创建单个教师或学生账号，支持后端按策略返回激活码。
   */
  async createAccount(data: CreateAccountRequest): Promise<CreateAccountResponse> {
    return this.client.post('/accounts', data)
  }

  /**
   * 更新账号基础资料，字段范围与后端学校管理员接口一致。
   */
  async updateAccount(accountId: string, data: UpdateAccountRequest): Promise<Account> {
    return this.client.patch(`/accounts/${accountId}`, data)
  }

  /**
   * 停用账号并由后端吊销相关会话。
   */
  async disableAccount(accountId: string): Promise<void> {
    return this.client.post(`/accounts/${accountId}/disable`)
  }

  /**
   * 启用已停用账号。
   */
  async enableAccount(accountId: string): Promise<void> {
    return this.client.post(`/accounts/${accountId}/enable`)
  }

  /**
   * 归档账号，适用于毕业或离校等租户内生命周期流转。
   */
  async archiveAccount(accountId: string): Promise<void> {
    return this.client.post(`/accounts/${accountId}/archive`)
  }

  /**
   * 恢复已归档账号为可用状态。
   */
  async restoreAccount(accountId: string): Promise<void> {
    return this.client.post(`/accounts/${accountId}/restore`)
  }

  /**
   * 注销账号，交由后端执行软删除和状态流转。
   */
  async cancelAccount(accountId: string): Promise<void> {
    return this.client.post(`/accounts/${accountId}/cancel`)
  }

  /**
   * 强制指定账号下线，吊销其所有服务端会话。
   */
  async forceLogoutAccount(accountId: string): Promise<void> {
    return this.client.post(`/accounts/${accountId}/force-logout`)
  }

  /**
   * 学校管理员重置指定账号密码，并可要求下次登录改密。
   */
  async resetAccountPassword(accountId: string, data: AdminResetPasswordRequest): Promise<void> {
    return this.client.post(`/accounts/${accountId}/reset-password`, data)
  }

  /**
   * 授予教师学校管理员角色。
   */
  async grantSchoolAdmin(accountId: string): Promise<void> {
    return this.client.post(`/accounts/${accountId}/grant-admin`)
  }

  /**
   * 撤销账号的学校管理员角色。
   */
  async revokeSchoolAdmin(accountId: string): Promise<void> {
    return this.client.post(`/accounts/${accountId}/revoke-admin`)
  }

  /**
   * 批量停用账号。
   */
  async batchDisableAccounts(data: BatchAccountIDsRequest): Promise<void> {
    return this.client.post('/accounts/batch/disable', data)
  }

  /**
   * 批量恢复账号为可用状态。
   */
  async batchRestoreAccounts(data: BatchAccountIDsRequest): Promise<void> {
    return this.client.post('/accounts/batch/restore', data)
  }

  /**
   * 上传师生导入文件并获取后端预览结果，预览状态由服务端持久化。
   */
  async previewAccountImport(targetType: 'teacher' | 'student', file: File, onProgress?: (progress: number) => void): Promise<ImportPreviewResponse> {
    const formData = new FormData()
    formData.append('type', targetType)
    formData.append('file', file)
    return this.client.postFormData('/accounts/import/preview', formData, onProgress)
  }

  /**
   * 提交已预览的师生导入批次。
   */
  async commitAccountImport(data: ImportCommitRequest): Promise<AccountImportCommitResponse> {
    return this.client.post('/accounts/import/commit', data)
  }

  /**
   * 下载教师或学生导入模板，文件名取自后端响应头。
   */
  async downloadAccountImportTemplate(params: { type: 'teacher' | 'student'; format?: ImportTemplateFormat }): Promise<AttachmentResponse> {
    return this.client.getAttachment('/accounts/import/template', params)
  }
  /**
   * 查询当前租户账号导入批次历史。
   */
  async listAccountImportBatches(): Promise<ImportBatch[]> {
    return this.client.get('/accounts/import/batches')
  }

  // ===== 组织架构（学校管理员） =====

  /**
   * 查询当前租户院系列表。
   */
  async listDepartments(): Promise<Department[]> {
    return this.client.get('/org/departments')
  }

  /**
   * 创建院系。
   */
  async createDepartment(data: DepartmentRequest): Promise<Department> {
    return this.client.post('/org/departments', data)
  }

  /**
   * 更新院系名称或编码。
   */
  async updateDepartment(id: string, data: DepartmentRequest): Promise<Department> {
    return this.client.patch(`/org/departments/${id}`, data)
  }

  /**
   * 删除院系，由后端校验是否仍被专业或账号引用。
   */
  async deleteDepartment(id: string): Promise<void> {
    return this.client.delete(`/org/departments/${id}`)
  }

  /**
   * 查询专业列表，可按院系过滤。
   */
  async listMajors(params?: { department_id?: string }): Promise<Major[]> {
    return this.client.get('/org/majors', params)
  }

  /**
   * 创建专业并绑定所属院系。
   */
  async createMajor(data: MajorRequest): Promise<Major> {
    return this.client.post('/org/majors', data)
  }

  /**
   * 更新专业信息。
   */
  async updateMajor(id: string, data: MajorRequest): Promise<Major> {
    return this.client.patch(`/org/majors/${id}`, data)
  }

  /**
   * 删除专业，由后端校验班级依赖。
   */
  async deleteMajor(id: string): Promise<void> {
    return this.client.delete(`/org/majors/${id}`)
  }

  /**
   * 查询班级列表，可按专业过滤。
   */
  async listClasses(params?: { major_id?: string }): Promise<Class[]> {
    return this.client.get('/org/classes', params)
  }

  /**
   * 查询班内在校学生名录（只含编号、姓名、学号）。
   * 教师挑选学生（如给实验小组分配成员）用这一条；账号目录 `/accounts` 是学校管理员能力，
   * 会带手机号掩码、账号状态与角色，教师端不该经它取人。
   */
  async listClassStudents(classId: string): Promise<ClassStudent[]> {
    return this.client.get(`/org/classes/${classId}/students`)
  }

  /**
   * 创建班级。
   */
  async createClass(data: ClassRequest): Promise<Class> {
    return this.client.post('/org/classes', data)
  }

  /**
   * 更新班级信息。
   */
  async updateClass(id: string, data: ClassRequest): Promise<Class> {
    return this.client.patch(`/org/classes/${id}`, data)
  }

  /**
   * 删除班级，由后端校验账号依赖。
   */
  async deleteClass(id: string): Promise<void> {
    return this.client.delete(`/org/classes/${id}`)
  }

  /**
   * 上传组织架构导入文件并获取预览结果。
   */
  async previewOrgImport(file: File, onProgress?: (progress: number) => void): Promise<ImportPreviewResponse> {
    const formData = new FormData()
    formData.append('file', file)
    return this.client.postFormData('/org/import/preview', formData, onProgress)
  }

  /**
   * 提交已预览的组织架构导入批次。
   */
  async commitOrgImport(data: ImportCommitRequest): Promise<AccountImportCommitResponse> {
    return this.client.post('/org/import/commit', data)
  }

  /**
   * 下载组织架构导入模板，文件名取自后端响应头。
   */
  async downloadOrgImportTemplate(params?: { format?: ImportTemplateFormat }): Promise<AttachmentResponse> {
    return this.client.getAttachment('/org/import/template', params)
  }

  /**
   * 按入学年份归档班级。
   */
  async archiveClasses(data: ArchiveClassesRequest): Promise<void> {
    return this.client.post('/org/classes/archive', data)
  }

  /**
   * 执行班级批量升级。
   */
  async promoteClasses(data: PromoteClassesRequest): Promise<void> {
    return this.client.post('/org/classes/promote', data)
  }

  // ===== 租户配置（学校管理员） =====

  /**
   * 读取当前租户配置。
   */
  async getTenantConfig(): Promise<Tenant> {
    return this.client.get('/tenant/config')
  }

  /**
   * 更新当前租户展示、认证和功能开关配置。
   */
  async updateTenantConfig(data: TenantConfigRequest): Promise<Tenant> {
    return this.client.patch('/tenant/config', data)
  }

  /**
   * 上传校徽,上传即生效并返回更新后的学校配置。
   * 不需要再提交一次引用:学校管理员上传时学校一定已存在,没有「对象先于记录」的时序问题,
   * 而两步提交一旦被放弃就会留下没有引用者的对象。
   */
  async uploadTenantLogo(
    file: File,
    onProgress?: (progress: number) => void
  ): Promise<Tenant> {
    const formData = new FormData()
    formData.append('file', file)
    return this.client.postFormData('/tenant/logo', formData, onProgress)
  }

  /**
   * 移除校徽,徽记位回落学校名首字。
   */
  async clearTenantLogo(): Promise<Tenant> {
    return this.client.delete('/tenant/logo')
  }

  /**
   * 读取学校品牌,供登录页在没有会话时显示学校名与校徽。
   * 不接受任何学校标识参数:平台托管部署返回空品牌,只有学校私有部署才有内容。
   */
  async getTenantBrand(): Promise<TenantBrand> {
    return this.client.get('/tenant/brand')
  }

  /**
   * 查询当前租户 SSO 配置列表，敏感字段由后端脱敏。
   */
  async listSSOConfigs(): Promise<SSOConfig[]> {
    return this.client.get('/tenant/sso')
  }

  /**
   * 创建或更新当前租户 SSO 配置。
   */
  async upsertSSOConfig(data: SSOConfigRequest): Promise<SSOConfig> {
    return this.client.put('/tenant/sso', data)
  }

  // ===== 租户管理（平台管理员） =====

  /**
   * 创建入驻申请
   */
  async createApplication(data: CreateApplicationRequest): Promise<TenantApplication> {
    return this.client.post('/platform/applications', data)
  }

  /**
   * 获取入驻申请列表
   */
  async getApplications(params?: { status?: ApplicationStatus }): Promise<TenantApplication[]> {
    return this.client.get('/platform/applications', params)
  }

  /**
   * 通过入驻申请
   */
  async approveApplication(applicationId: string, data: ReviewApplicationRequest): Promise<{ tenant: Tenant; activation_code?: string }> {
    return this.client.post(`/platform/applications/${applicationId}/approve`, data)
  }

  /**
   * 驳回入驻申请
   */
  async rejectApplication(applicationId: string, data: ReviewApplicationRequest): Promise<void> {
    return this.client.post(`/platform/applications/${applicationId}/reject`, data)
  }

  /**
   * 获取租户列表
   */
  async getTenants(params?: { page?: number; size?: number }): Promise<PaginatedResponse<Tenant>> {
    return this.client.get('/platform/tenants', params)
  }

  /**
   * 获取租户详情
   */
  async getTenant(tenantId: string): Promise<Tenant> {
    return this.client.get(`/platform/tenants/${tenantId}`)
  }

  /**
   * 更新租户状态
   */
  async updateTenant(tenantId: string, data: UpdateTenantStatusRequest): Promise<Tenant> {
    return this.client.patch(`/platform/tenants/${tenantId}`, data)
  }
}
