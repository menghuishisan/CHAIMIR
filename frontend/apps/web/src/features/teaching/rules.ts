// teaching rules 文件维护 M6 前端领域判断规则。

import { LessonContentType } from '@chaimir/api-client'

const MATERIAL_CONTENT_TYPES: ReadonlySet<LessonContentType> = new Set([
  LessonContentType.VIDEO,
  LessonContentType.ATTACHMENT,
])

/** isLessonMaterialType 判断课时是否需要文件投放。 */
export function isLessonMaterialType(type: LessonContentType): boolean {
  return MATERIAL_CONTENT_TYPES.has(type)
}
