// reference labels 文件维护跨模块来源引用的统一用户文案。

/** sourceReferenceLabel 将跨模块来源引用转换为用户可读的业务记录名称。 */
export function sourceReferenceLabel(sourceRef: string): string {
  const parts = sourceRef.split(':').filter(Boolean)
  const resource = parts.at(-2) || ''
  const id = parts.at(-1) || ''
  const label = ({
    'submission-item': '课程作业提交', submission: '提交记录', experiment: '实验记录', contest: '竞赛记录',
  } as Record<string, string>)[resource] || '业务记录'
  return id ? `${label} · 编号 ${id}` : label
}

/** auditActionLabel 将审计动作键转换为可扫描的业务动作。 */
export function auditActionLabel(action: string): string {
  const labels: Record<string, string> = {
    create: '创建', update: '更新', publish: '发布', start: '开始', end: '结束', archive: '归档',
    delete: '删除', disable: '停用', enable: '启用', approve: '通过审核', reject: '驳回',
    submit: '提交', finalize: '固化', prevalidate: '预验证', lock: '锁定', unlock: '解锁',
    import: '导入', export: '导出', login: '登录', logout: '退出登录', recycle: '回收资源',
  }
  const segments = action.split('.').filter(Boolean)
  const operation = labels[segments.at(-1) || ''] || '执行操作'
  const scope = segments.slice(0, -1).map(auditSegmentLabel).filter(Boolean).join(' / ')
  return scope ? `${scope} · ${operation}` : operation
}

/** auditTargetLabel 将审计目标键转换为业务对象名称。 */
export function auditTargetLabel(target: string): string {
  return auditSegmentLabel(target) || '业务对象'
}

/** auditSegmentLabel 转换常见模块和资源片段。 */
function auditSegmentLabel(segment: string): string {
  const labels: Record<string, string> = {
    identity: '账号与租户', teaching: '课程教学', content: '内容资源', experiment: '实验', contest: '竞赛',
    grade: '成绩', sandbox: '实验环境', judge: '判题', sim: '仿真', notify: '通知', admin: '平台治理',
    course: '课程', assignment: '作业', account: '账号', application: '入驻申请', problem: '题目',
    vuln_problem: '漏洞题', cheat_record: '违规记录', review: '审核记录', alert: '告警', config: '配置',
  }
  return labels[segment] || segment.replace(/[_-]+/g, ' ')
}
