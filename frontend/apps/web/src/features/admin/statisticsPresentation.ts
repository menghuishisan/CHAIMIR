// statisticsPresentation 文件维护管理看板趋势图的指标分组。

/** StatisticsMetricGroup 描述一张趋势图展示的一组指标。 */
export interface StatisticsMetricGroup {
  id: string
  title: string
  keys: string[]
}

/** STATISTICS_METRIC_GROUPS 按业务语义拆分趋势图,每组不超过图表系列上限。 */
export const STATISTICS_METRIC_GROUPS: StatisticsMetricGroup[] = [
  { id: 'account', title: '账号', keys: ['account_count', 'active_account_count', 'new_account_count'] },
  { id: 'tenant', title: '学校', keys: ['tenant_count'] },
  {
    id: 'teaching',
    title: '教学',
    keys: ['course_count', 'active_course_count', 'experiment_count', 'active_instance_count'],
  },
  {
    id: 'contest',
    title: '竞赛与判题',
    keys: ['contest_count', 'active_contest_count', 'submission_count', 'judge_task_count'],
  },
  { id: 'resource', title: '资源与时长', keys: ['active_sandbox_count', 'learning_duration_sec'] },
]
