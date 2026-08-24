// Admin API 文件定义 M9 管理后台前端唯一调用入口。

import { ApiClient, encodePathSegment } from '../client'
import type { AdminScope, AlertLevel, AlertStatus, BackupStatus } from '../constants/admin'
import type { PaginatedResponse } from '../types/common'
import type {
  AlertEvent,
  AlertEventRequest,
  AlertRule,
  AlertRuleRequest,
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
} from '../types/admin'
import type { TransferTask } from '../types/transfer'

/**
 * AdminApi 封装 M9 文档定义的管理后台 HTTP API,不保留旧路径或过渡别名。
 */
export class AdminApi {
  /**
   * constructor 注入统一 ApiClient,确保管理后台接口共用鉴权、trace_id 和错误处理。
   */
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

  // queryAudit 查询共享审计日志。
  async queryAudit(params?: AuditQueryParams): Promise<AuditQueryResult> {
    return this.client.get('/admin/audit', params)
  }

  // exportAudit 创建审计导出任务。
  async exportAudit(params?: AuditQueryParams): Promise<TransferTask> {
    return this.client.get('/admin/audit/export', params)
  }

  // listConfigs 查询系统配置列表。
  async listConfigs(params?: { scope?: AdminScope }): Promise<SystemConfig[]> {
    return this.client.get('/admin/configs', params)
  }

  /**
   * getConfig 按 key 读取单个系统配置。
   * 返回的是脱敏值与当前 version —— 回滚要带 version(乐观锁),
   * 故「进详情 → 读 version → 回滚」现在是一条完整链路,不必再拉全量配置列表。
   */
  async getConfig(key: string, params?: { scope?: AdminScope }): Promise<SystemConfig> {
    return this.client.get(`/admin/configs/${encodePathSegment(key)}`, params)
  }

  // updateConfig 按配置 key 和乐观锁版本更新系统配置。
  async updateConfig(key: string, data: ConfigUpdateRequest): Promise<SystemConfig> {
    return this.client.put(`/admin/configs/${encodePathSegment(key)}`, data)
  }

  // listConfigHistory 查询配置变更历史。
  async listConfigHistory(
    key: string,
    params?: { scope?: AdminScope; tenant_id?: string; page?: number; size?: number },
  ): Promise<PaginatedResponse<ConfigChangeLog>> {
    return this.client.get(`/admin/configs/${encodePathSegment(key)}/history`, params)
  }

  // rollbackConfig 把配置回退到指定历史记录的旧值。
  async rollbackConfig(key: string, data: ConfigRollbackRequest): Promise<SystemConfig> {
    return this.client.post(`/admin/configs/${encodePathSegment(key)}/rollback`, data)
  }

  // listAlertRules 查询业务级告警规则。
  async listAlertRules(params?: { scope?: AdminScope }): Promise<AlertRule[]> {
    return this.client.get('/admin/alert-rules', params)
  }

  // createAlertRule 创建业务级告警规则。
  async createAlertRule(data: AlertRuleRequest): Promise<AlertRule> {
    return this.client.post('/admin/alert-rules', data)
  }

  // updateAlertRule 更新业务级告警规则。
  async updateAlertRule(ruleId: string, data: AlertRuleRequest): Promise<AlertRule> {
    return this.client.patch(`/admin/alert-rules/${encodePathSegment(ruleId)}`, data)
  }

  /**
   * listAlertEvents 查询业务级告警事件。
   * status 与 level 都以 0 表示不限,由服务端过滤,total 与筛选同口径 ——
   * 按级别统计必须走这个参数,不能用当前页切片数(§6.5.4)。
   */
  async listAlertEvents(params?: {
    status?: AlertStatus
    level?: AlertLevel
    page?: number
    size?: number
  }): Promise<PaginatedResponse<AlertEvent>> {
    return this.client.get('/admin/alert-events', params)
  }

  // handleAlertEvent 处理或忽略一条待处理告警。
  async handleAlertEvent(eventId: string, data: AlertEventRequest): Promise<AlertEvent> {
    return this.client.post(`/admin/alert-events/${encodePathSegment(eventId)}/handle`, data)
  }

  // monitoringPanels 读取外接监控系统安全嵌入入口。
  async monitoringPanels(): Promise<MonitoringPanel[]> {
    return this.client.get('/admin/platform/monitoring/panels')
  }

  // listBackups 查询受控运维任务写入的备份记录。结果筛选由服务端执行,total 与筛选同口径。
  async listBackups(params?: {
    status?: BackupStatus
    page?: number
    size?: number
  }): Promise<PaginatedResponse<BackupRecord>> {
    return this.client.get('/admin/platform/backups', params)
  }
}
