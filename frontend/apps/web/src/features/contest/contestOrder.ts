// 学生与教师列表的任务优先级排序，独立于正式状态筛选。
import { ContestStatus } from '@chaimir/api-client'

/** 排序用到的赛事字段 */
export interface OrderContest {
  id: string
  status: number
  signup_end: string
  end_at: string
}

/** 无效时间排在当前分组末尾。 */
function timeOf(value: string): number {
  const ms = new Date(value).getTime()
  return Number.isFinite(ms) ? ms : Number.MAX_SAFE_INTEGER
}

/** sortStudentContests 按学生关心的先后排序,返回新数组 */
export function sortStudentContests<T extends OrderContest>(list: readonly T[], joined: ReadonlySet<string>): T[] {
  const rank = (contest: T): number => {
    if (contest.status === ContestStatus.RUNNING || contest.status === ContestStatus.FROZEN) return joined.has(contest.id) ? 0 : 2
    if (contest.status === ContestStatus.SIGNUP) return 1
    if (contest.status === ContestStatus.ENDED || contest.status === ContestStatus.ARCHIVED) return 3
    return 4
  }
  return [...list].sort((a, b) => {
    const diff = rank(a) - rank(b)
    if (diff !== 0) return diff
    if (a.status === ContestStatus.SIGNUP) {
      const mine = Number(joined.has(b.id)) - Number(joined.has(a.id))
      return mine !== 0 ? mine : timeOf(a.signup_end) - timeOf(b.signup_end)
    }
    if (a.status === ContestStatus.ENDED || a.status === ContestStatus.ARCHIVED) return timeOf(b.end_at) - timeOf(a.end_at)
    return timeOf(a.end_at) - timeOf(b.end_at)
  })
}

/** 教师排序用到的赛事字段 */
export interface TeacherOrderContest extends OrderContest {
  start_at: string
}

/**
 * sortTeacherContests 按教师接下来要管的先后排序,返回新数组:
 * 进行中(含封榜,先结束的在前)→ 报名中(先开赛的在前)→ 草稿(发布期限即报名截止近的在前)
 * → 已结束待归档 → 已归档(后两组最近结束的在前)。
 */
export function sortTeacherContests<T extends TeacherOrderContest>(list: readonly T[]): T[] {
  const rank = (status: number): number => {
    if (status === ContestStatus.RUNNING || status === ContestStatus.FROZEN) return 0
    if (status === ContestStatus.SIGNUP) return 1
    if (status === ContestStatus.DRAFT) return 2
    if (status === ContestStatus.ENDED) return 3
    if (status === ContestStatus.ARCHIVED) return 4
    return 5
  }
  return [...list].sort((a, b) => {
    const diff = rank(a.status) - rank(b.status)
    if (diff !== 0) return diff
    switch (rank(a.status)) {
      case 0:
        return timeOf(a.end_at) - timeOf(b.end_at)
      case 1:
        return timeOf(a.start_at) - timeOf(b.start_at)
      case 2:
        return timeOf(a.signup_end) - timeOf(b.signup_end)
      default:
        return timeOf(b.end_at) - timeOf(a.end_at)
    }
  })
}
