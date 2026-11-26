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
  Dashboard,
  MonitoringPanel,
  Statistics,
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
  async getPlatformDashboard(): Promise<Dashboard> {
    return this.client.get('/admin/platform/dashboard')
  }
  async getSchoolDashboard(): Promise<Dashboard> {
    return this.client.get('/admin/school/dashboard')
  }
  async getPlatformStatistics(params: { from: string; to: string }): Promise<Statistics[]> {
    return this.client.get('/admin/platform/statistics', params)
  }
  async getSchoolStatistics(params: { from: string; to: string }): Promise<Statistics[]> {
    return this.client.get('/admin/school/statistics', params)
  }
  async queryAudit(params?: AuditQueryParams): Promise<AuditQueryResult> {
    return this.client.get('/admin/audit', params)
  }
  async exportAudit(params?: AuditQueryParams): Promise<TransferTask> {
    return this.client.get('/admin/audit/export', params)
  }
  async listAlertRules(params?: { scope?: AdminScope }): Promise<AlertRule[]> {
    return this.client.get('/admin/alert-rules', params)
  }
  async createAlertRule(data: AlertRuleRequest): Promise<AlertRule> {
    return this.client.post('/admin/alert-rules', data)
  }
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
  async handleAlertEvent(eventId: string, data: AlertEventRequest): Promise<AlertEvent> {
    return this.client.post(`/admin/alert-events/${encodePathSegment(eventId)}/handle`, data)
  }
  async monitoringPanels(): Promise<MonitoringPanel[]> {
    return this.client.get('/admin/platform/monitoring/panels')
  }
  async listBackups(params?: {
    status?: BackupStatus
    page?: number
    size?: number
  }): Promise<PaginatedResponse<BackupRecord>> {
    return this.client.get('/admin/platform/backups', params)
  }
}
