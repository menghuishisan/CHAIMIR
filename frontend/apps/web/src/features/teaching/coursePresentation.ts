// coursePresentation 文件维护课程类型默认封面的展示配置。

import type { CoverAccent } from '@chaimir/ui'
import { CourseType } from '@chaimir/api-client'

const COURSE_TYPE_COVERS: Record<CourseType, { glyph: string; accent: CoverAccent }> = {
  [CourseType.THEORY]: { glyph: '理论', accent: 'blue' },
  [CourseType.LAB]: { glyph: '实验', accent: 'jade' },
  [CourseType.MIXED]: { glyph: '兼修', accent: 'cinnabar' },
  [CourseType.PROJECT]: { glyph: '项目', accent: 'graphite' },
}

/** courseTypeCover 返回课程类型默认封面的题识与语义色。 */
export function courseTypeCover(type: CourseType): { glyph: string; accent: CoverAccent } {
  return COURSE_TYPE_COVERS[type]
}
