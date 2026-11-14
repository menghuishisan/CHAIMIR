// gradeLevel.ts 把课程总评换算成等级与绩点(设计 §7.2 成绩中心:成绩表列「等级、绩点」)。
// 成绩接口只回总评分数;学校的等级映射(默认等级配置)学生可读,换算规则与后端 ComputeGPA 一致:
// 按分数阈值降序,取第一个「分数 ≥ 阈值」的档。纯函数,供 node:test 直接运行。

/** 等级映射里的一档(与 @chaimir/api-client 的 LevelRule 同形) */
export interface LevelRuleInput {
  min: number
  grade: string
  gpa: number
}

/** levelForScore 给出分数对应的等级与绩点;没有配置或没有命中时返回 undefined */
export function levelForScore(score: number, mapping: readonly LevelRuleInput[]): { grade: string; gpa: number } | undefined {
  const ordered = [...mapping].sort((a, b) => b.min - a.min)
  const rule = ordered.find((item) => score >= item.min)
  return rule ? { grade: rule.grade, gpa: rule.gpa } : undefined
}
