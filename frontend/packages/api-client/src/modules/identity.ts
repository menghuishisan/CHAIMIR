// Identity API：认证、授权、用户管理
// 对应后端 M1 模块

import { ApiClient } from '../client'
import type {
  LoginPlatformRequest,
  LoginPhoneRequest,
  LoginNoRequest,
  LoginSMSRequest,
  SendSMSRequest,
  RefreshRequest,
  PasswordResetRequest,
  ActivateRequest,
  LoginResponse,
  Account,
  ChangePasswordRequest,
  ChangePhoneRequest,
  Session,
  AuditLog,
  PaginatedResponse,
  CreateApplicationRequest,
  Tenant,
} from '../types'

export class IdentityApi {
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
   * 刷新 Token
   */
  async refreshToken(data: RefreshRequest): Promise<LoginResponse> {
    return this.client.post('/auth/refresh', data)
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
  async activate(data: ActivateRequest): Promise<LoginResponse> {
    return this.client.post('/auth/activate', data)
  }

  /**
   * 登出
   */
  async logout(): Promise<void> {
    return this.client.post('/auth/logout')
  }

  // ===== 当前用户 =====

  /**
   * 获取当前用户信息
   */
  async getMe(): Promise<Account> {
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

  // ===== 审计日志 =====

  /**
   * 查询审计日志（管理员）
   */
  async getAuditLogs(params: {
    actor_id?: string
    action?: string
    target_type?: string
    from?: string
    to?: string
    page?: number
    size?: number
  }): Promise<PaginatedResponse<AuditLog>> {
    return this.client.get('/audit', params)
  }

  // ===== 租户管理（平台管理员） =====

  /**
   * 创建入驻申请
   */
  async createApplication(data: CreateApplicationRequest): Promise<{ application_id: string }> {
    return this.client.post('/platform/applications', data)
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
}
