// learningPath.ts 把课程大纲整理成学习路径(设计 §7.2):课时按章序、再按章内序排列;
// 「下一个课时」先取第一个学习中,再取第一个未开始,全部学完时为空;课时页的「上一节 / 下一节」由同一顺序计算。
// 进度缺失的课时(从没打开过)视为未开始。纯函数,只认排序与进度用到的字段,供 node:test 直接运行。

/** 进度状态取值,与 @chaimir/api-client 的 ProgressStatus 一致(本文件不在运行时导入它) */
const STATUS_NOT_STARTED = 1
const STATUS_IN_PROGRESS = 2

/** 计算学习路径用到的大纲字段 */
export interface PathInput {
  chapters: readonly { id: string; sort: number }[]
  lessons: readonly { id: string; chapter_id: string; sort: number }[]
  progress: readonly { lesson_id: string; status: number; duration_sec: number }[]
}

/** 学习路径里的一节课 */
export interface PathLesson {
  lessonId: string
  chapterId: string
  status: number
  durationSec: number
}

/** orderedLessons 按章序、再按章内序排列课时,并对上各自的进度 */
export function orderedLessons(input: PathInput): PathLesson[] {
  const chapterSort = new Map(input.chapters.map((chapter) => [chapter.id, chapter.sort]))
  const progressByLesson = new Map(input.progress.map((item) => [item.lesson_id, item]))
  return [...input.lessons]
    .sort((a, b) => {
      const chapterDiff = (chapterSort.get(a.chapter_id) ?? 0) - (chapterSort.get(b.chapter_id) ?? 0)
      return chapterDiff !== 0 ? chapterDiff : a.sort - b.sort
    })
    .map((lesson) => {
      const progress = progressByLesson.get(lesson.id)
      return {
        lessonId: lesson.id,
        chapterId: lesson.chapter_id,
        status: progress ? progress.status : STATUS_NOT_STARTED,
        durationSec: progress ? progress.duration_sec : 0,
      }
    })
}

/** pickNextLesson 给出「下一步」卡片要推荐的课时;全部学完时返回 null */
export function pickNextLesson(rows: readonly PathLesson[]): PathLesson | null {
  return (
    rows.find((row) => row.status === STATUS_IN_PROGRESS) ??
    rows.find((row) => row.status === STATUS_NOT_STARTED) ??
    null
  )
}

/** adjacentLessons 给出某节课的上一节与下一节;不在路径里时两边都为 null */
export function adjacentLessons(
  rows: readonly PathLesson[],
  lessonId: string,
): { previous: string | null; next: string | null } {
  const index = rows.findIndex((row) => row.lessonId === lessonId)
  if (index < 0) return { previous: null, next: null }
  return {
    previous: index > 0 ? rows[index - 1].lessonId : null,
    next: index < rows.length - 1 ? rows[index + 1].lessonId : null,
  }
}
