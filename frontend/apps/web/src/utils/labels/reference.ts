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
