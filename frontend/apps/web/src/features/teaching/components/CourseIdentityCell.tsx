// teaching 课程身份单元格:统一多角色课程列表的封面、名称与学期信息。

import type { ReactNode } from 'react'
import type { Course } from '@chaimir/api-client'
import { CoverImage } from '@chaimir/ui'
import { courseTypeCover } from '../coursePresentation'

interface CourseIdentityCellProps {
  course: Course
  details?: ReactNode
}

/** CourseIdentityCell 呈现课程列表中稳定的封面与主身份信息。 */
export function CourseIdentityCell({ course, details }: CourseIdentityCellProps) {
  const cover = courseTypeCover(course.type)

  return (
    <div className="flex min-w-0 items-center gap-3">
      <CoverImage
        id={course.id}
        name={course.name}
        glyph={cover.glyph}
        accent={cover.accent}
        className="w-28 shrink-0"
      />
      <div className="min-w-0">
        <div className="truncate font-medium text-ink">{course.name}</div>
        <div className="truncate text-xs text-ink-sub">{details ?? course.semester}</div>
      </div>
    </div>
  )
}
