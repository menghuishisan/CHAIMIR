// 课程总评换算沿用学校等级映射，与后端 ComputeGPA 的阈值规则一致。

import type { LevelRule } from '@chaimir/api-client'

/** levelForScore 给出分数对应的等级与绩点;没有配置或没有命中时返回 undefined */
export function levelForScore(score: number, mapping: readonly LevelRule[]): { grade: string; gpa: number } | undefined {
  const ordered = [...mapping].sort((a, b) => b.min - a.min)
  const rule = ordered.find((item) => score >= item.min)
  return rule ? { grade: rule.grade, gpa: rule.gpa } : undefined
}
