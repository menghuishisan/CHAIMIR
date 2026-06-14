// Admin API：管理后台
// 对应后端 M9 模块

import { ApiClient } from '../client'
import type {
  SystemConfig,
  ConfigUpdateRequest,
  AlertRule,
  AlertRuleRequest,
  AlertEvent,
  Statistics,
  BackupRecord,
  PaginatedResponse,
} from '../types'

export class AdminApi {
  constructor(private client: ApiClient) {}

  // ===== 系统配置 =====

  /**
   * 获取配置列表
   */
  async getConfigs(params?: { scope?: number; tenant_id?: string }): Promise<SystemConfig[]> {
    return this.client.get('/admin/configs', params)
  }

  /**
   * 获取配置详情
   */
  async getConfig(configId: string): Promise<SystemConfig> {
    return this.client.get(`/admin/configs/${configId}`)
  }

  /**
   * 更新配置
   */
  async updateConfig(configId: string, data: ConfigUpdateRequest): Promise<SystemConfig> {
    return this.client.put(`/admin/configs/${configId}`, data)
  }

  /**
   * 回滚配置
   */
  async rollbackConfig(configId: string, changeLogId: string): Promise<void> {
    return this.client.post(`/admin/configs/${configId}/rollback`, { change_log_id: changeLogId })
  }

  /**
   * 获取配置变更历史
   */
  async getConfigChangeLogs(configId: string): Promise<any[]> {
    return this.client.get(`/admin/configs/${configId}/changelog`)
  }

  // ===== 告警规则 =====

  /**
   * 获取告警规则列表
   */
  async getAlertRules(params?: { scope?: number; tenant_id?: string }): Promise<AlertRule[]> {
    return this.client.get('/admin/alert-rules', params)
  }

  /**
   * 创建告警规则
   */
  async createAlertRule(data: AlertRuleRequest): Promise<AlertRule> {
    return this.client.post('/admin/alert-rules', data)
  }

  /**
   * 更新告警规则
   */
  async updateAlertRule(ruleId: string, data: AlertRuleRequest): Promise<AlertRule> {
    return this.client.put(`/admin/alert-rules/${ruleId}`, data)
  }

  /**
   * 删除告警规则
   */
  async deleteAlertRule(ruleId: string): Promise<void> {
    return this.client.delete(`/admin/alert-rules/${ruleId}`)
  }

  /**
   * 获取告警事件列表
   */
  async getAlertEvents(params?: {
    rule_id?: string
    status?: number
    level?: number
    page?: number
    size?: number
  }): Promise<PaginatedResponse<AlertEvent>> {
    return this.client.get('/admin/alert-events', params)
  }

  /**
   * 处理告警事件
   */
  async handleAlertEvent(eventId: string, data: { status: number }): Promise<void> {
    return this.client.post(`/admin/alert-events/${eventId}/handle`, data)
  }

  // ===== 运营统计 =====

  /**
   * 获取统计数据
   */
  async getStatistics(params: {
    scope: number
    tenant_id?: string
    metric: string
    from: string
    to: string
    granularity?: string
  }): Promise<Statistics[]> {
    return this.client.get('/admin/statistics', params)
  }

  /**
   * 获取看板数据（综合）
   */
  async getDashboard(params?: { scope?: number; tenant_id?: string }): Promise<Record<string, any>> {
    return this.client.get('/admin/dashboard', params)
  }

  // ===== 备份管理 =====

  /**
   * 获取备份记录列表
   */
  async getBackupRecords(params?: {
    type?: number
    status?: number
    page?: number
    size?: number
  }): Promise<PaginatedResponse<BackupRecord>> {
    return this.client.get('/admin/backups', params)
  }

  /**
   * 手动触发备份
   */
  async triggerBackup(data: { type: number }): Promise<{ backup_id: string }> {
    return this.client.post('/admin/backups/trigger', data)
  }

  /**
   * 下载备份文件
   */
  async downloadBackup(backupId: string): Promise<void> {
    const filename = `backup_${backupId}.tar.gz`
    return this.client.download(`/admin/backups/${backupId}/download`, filename)
  }
}
