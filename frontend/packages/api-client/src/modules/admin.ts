// Admin API 文件定义 M9 管理后台前端唯一调用入口。

import { ApiClient } from '../client'
import type {
  AlertEvent,
  AlertEventRequest,
  AlertRule,
  AlertRuleRequest,
  AuditExportTask,
  AuditQueryParams,
  AuditQueryResult,
  BackupRecord,
  ConfigChangeLog,
  ConfigRollbackRequest,
  ConfigUpdateRequest,
  Dashboard,
  MonitoringPanel,
  Statistics,
  SystemConfig,
  TenantApplicationSummary,
  TenantSummary,
} from '../types'

// AdminApi 封装 M9 文档定义的管理后台 HTTP API,不保留旧路径或兼容别名。
export class AdminApi {
  constructor(private client: ApiClient) {}

  // getPlatformDashboard 读取平台级聚合看板。
  async getPlatformDashboard(): Promise<Dashboard> {
    return this.client.get('/admin/platform/dashboard')
  }

  // getSchoolDashboard 读取当前学校聚合看板。
  async getSchoolDashboard(): Promise<Dashboard> {
    return this.client.get('/admin/school/dashboard')
  }

  // getPlatformStatistics 读取平台级统计快照。
  async getPlatformStatistics(params: { from: string; to: string }): Promise<Statistics[]> {
    return this.client.get('/admin/platform/statistics', params)
  }

  // getSchoolStatistics 读取当前学校统计快照。
  async getSchoolStatistics(params: { from: string; to: string }): Promise<Statistics[]> {
    return this.client.get('/admin/school/statistics', params)
  }

  // listTenants 读取平台租户摘要列表。
  async listTenants(): Promise<TenantSummary[]> {
    return this.client.get('/admin/platform/tenants')
  }

  // listApplications 读取学校入驻申请摘要列表。
  async listApplications(params?: { status?: number }): Promise<TenantApplicationSummary[]> {
    return this.client.get('/admin/platform/applications', params)
  }

  // queryAudit 查询共享审计日志。
  async queryAudit(params?: AuditQueryParams): Promise<AuditQueryResult> {
    return this.client.get('/admin/audit', params)
  }

  // exportAudit 创建审计导出任务。
  async exportAudit(params?: AuditQueryParams): Promise<AuditExportTask> {
    return this.client.get('/admin/audit/export', params)
  }

  // listConfigs 查询系统配置列表。
  async listConfigs(params?: { scope?: number }): Promise<SystemConfig[]> {
    return this.client.get('/admin/configs', params)
  }

  // updateConfig 按配置 key 和乐观锁版本更新系统配置。
  async updateConfig(key: string, data: ConfigUpdateRequest): Promise<SystemConfig> {
    return this.client.put(`/admin/configs/${encodeURIComponent(key)}`, data)
  }

  // listConfigHistory 查询配置变更历史。
  async listConfigHistory(
    key: string,
    params?: { scope?: number; tenant_id?: string; page?: number; size?: number },
  ): Promise<ConfigChangeLog[]> {
    return this.client.get(`/admin/configs/${encodeURIComponent(key)}/history`, params)
  }

  // rollbackConfig 把配置回退到指定历史记录的旧值。
  async rollbackConfig(key: string, data: ConfigRollbackRequest): Promise<SystemConfig> {
    return this.client.post(`/admin/configs/${encodeURIComponent(key)}/rollback`, data)
  }

  // listAlertRules 查询业务级告警规则。
  async listAlertRules(params?: { scope?: number }): Promise<AlertRule[]> {
    return this.client.get('/admin/alert-rules', params)
  }

  // createAlertRule 创建业务级告警规则。
  async createAlertRule(data: AlertRuleRequest): Promise<AlertRule> {
    return this.client.post('/admin/alert-rules', data)
  }

  // updateAlertRule 更新业务级告警规则。
  async updateAlertRule(ruleId: string, data: AlertRuleRequest): Promise<AlertRule> {
    return this.client.patch(`/admin/alert-rules/${ruleId}`, data)
  }

  // listAlertEvents 查询业务级告警事件。
  async listAlertEvents(params?: { status?: number; page?: number; size?: number }): Promise<AlertEvent[]> {
    return this.client.get('/admin/alert-events', params)
  }

  // handleAlertEvent 处理或忽略一条待处理告警。
  async handleAlertEvent(eventId: string, data: AlertEventRequest): Promise<AlertEvent> {
    return this.client.post(`/admin/alert-events/${eventId}/handle`, data)
  }

  // monitoringPanels 读取外接监控系统安全嵌入入口。
  async monitoringPanels(): Promise<MonitoringPanel[]> {
    return this.client.get('/admin/platform/monitoring/panels')
  }

  // listBackups 查询受控运维任务写入的备份记录。
  async listBackups(params?: { page?: number; size?: number }): Promise<BackupRecord[]> {
    return this.client.get('/admin/platform/backups', params)
  }
}
