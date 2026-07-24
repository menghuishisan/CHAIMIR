// admin labels 文件维护平台治理、告警、备份和管理指标文案。

import { AlertStatus, BackupStatus, BackupType } from '@chaimir/api-client'
import { labelFromMap } from './map'

const ADMIN_METRIC_LABELS: Record<string, string> = {
  tenant_count: '入驻学校', account_count: '账号总数', teacher_count: '教师账号', student_count: '学生账号',
  active_account_count: '活跃账号', pending_apply_count: '待审核申请', course_count: '课程总数',
  active_course_count: '活跃课程', learning_duration_sec: '学习时长', experiment_count: '实验总数',
  active_instance_count: '活跃实验', contest_count: '竞赛总数', active_contest_count: '进行中竞赛',
  participant_count: '参赛人数', active_sandbox_count: '运行中沙箱', max_concurrent_sandbox: '沙箱并发上限',
  max_cpu: 'CPU 上限', max_memory_mb: '内存上限', result: '结果', stage: '阶段', trace_id: '报障编号',
}

/** announcementScopeLabel 返回公告覆盖范围文案。 */
export function announcementScopeLabel(scope: number): string {
  return labelFromMap(scope, { 1: '全平台可见', 2: '全校可见', 3: '按角色可见' }, '未识别的公告范围')
}

/** alertStatusLabel 返回告警事件处理状态文案。 */
export function alertStatusLabel(status: AlertStatus): string {
  return labelFromMap(status, { [AlertStatus.PENDING]: '待处理', [AlertStatus.HANDLED]: '已处理', [AlertStatus.IGNORED]: '已忽略' }, String(status))
}

/** alertLevelLabel 返回告警严重程度文案。 */
export function alertLevelLabel(level: number): string {
  return labelFromMap(level, { 1: '一级（紧急）', 2: '二级（重要）', 3: '三级（提醒）' }, '未识别的告警级别')
}

/** systemConfigLabel 返回平台配置键的管理端文案。 */
export function systemConfigLabel(key: string): string {
  return labelFromMap(key, { maintenance_mode: '平台维护模式' }, key)
}

/** backupTypeLabel 返回备份类型文案。 */
export function backupTypeLabel(type: BackupType): string {
  return labelFromMap(type, { [BackupType.FULL]: '全量备份' }, '未知')
}

/** backupStatusLabel 返回备份任务状态文案。 */
export function backupStatusLabel(status: BackupStatus): string {
  return labelFromMap(status, { [BackupStatus.RUNNING]: '进行中', [BackupStatus.SUCCEEDED]: '已完成', [BackupStatus.FAILED]: '失败' }, '未知')
}

/** adminMetricLabel 返回管理统计和自检详情的用户向指标名称。 */
export function adminMetricLabel(key: string): string {
  return ADMIN_METRIC_LABELS[key] || '其他指标'
}
