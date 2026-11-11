// LessonIcons 课时状态图标与课时类型图标:课程详情的学习路径与课时页的目录共用一套,两处看到的同一个状态长得一样。

import { CheckCircle2, CircleDashed, FileText, FlaskConical, Paperclip, PlayCircle, Radar, type LucideIcon } from 'lucide-react'
import { LessonContentType, ProgressStatus } from '@chaimir/api-client'
import { Icon } from '@chaimir/ui'
import { progressStatusLabel } from '../../../utils/labels/teaching'

/** 每种课时的图标 */
export const LESSON_TYPE_ICON: Record<LessonContentType, LucideIcon> = {
  [LessonContentType.VIDEO]: PlayCircle,
  [LessonContentType.MARKDOWN]: FileText,
  [LessonContentType.ATTACHMENT]: Paperclip,
  [LessonContentType.EXPERIMENT]: FlaskConical,
  [LessonContentType.SIMULATION]: Radar,
}

/** LessonStatusIcon 课时状态图标:已学完(对勾)、学习中(实心圈)、未开始(虚线圈);状态同时用读屏文字表达 */
export function LessonStatusIcon({ status }: { status: ProgressStatus }) {
  const label = progressStatusLabel(status)
  if (status === ProgressStatus.DONE) {
    return <Icon icon={CheckCircle2} size="md" className="text-primary" label={label} />
  }
  if (status === ProgressStatus.IN_PROGRESS) {
    return (
      <span role="img" aria-label={label} className="flex size-4.5 shrink-0 items-center justify-center rounded-full border-2 border-primary">
        <span className="size-1.5 rounded-full bg-primary" />
      </span>
    )
  }
  return <Icon icon={CircleDashed} size="md" className="text-ink-faint" label={label} />
}
