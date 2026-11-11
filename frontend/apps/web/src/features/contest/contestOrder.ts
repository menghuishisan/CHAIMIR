// contestOrder.ts 学生竞赛列表(设计 §7.2 竞赛参赛)与教师赛事列表(§7.4 赛事组织)的分段与排序。
// 分段:报名中 / 进行中(含封榜)/ 已结束(含归档);接口的状态筛选只接受单个状态,故分段在前端做。
// 排序:我已报名且进行中的最前 → 报名中(已报名的在前,再按报名截止先后)→ 其余进行中(先结束的在前)
// → 已结束(最近结束的在前)。纯函数,供 node:test 直接运行:不使用 enum,不在运行时导入 @chaimir/*。

/** 赛事状态取值,与 @chaimir/api-client 的 ContestStatus 一致 */
const STATUS_DRAFT = 1
const STATUS_SIGNUP = 2
const STATUS_RUNNING = 3
const STATUS_FROZEN = 4
const STATUS_ENDED = 5
const STATUS_ARCHIVED = 6

export type ContestSegment = 'signup' | 'running' | 'ended'

/** 排序用到的赛事字段 */
export interface OrderContest {
  id: string
  status: number
  signup_end: string
  end_at: string
}

/** contestSegmentOf 赛事状态所属的分段;草稿不属于任何分段 */
export function contestSegmentOf(status: number): ContestSegment | undefined {
  if (status === STATUS_SIGNUP) return 'signup'
  if (status === STATUS_RUNNING || status === STATUS_FROZEN) return 'running'
  if (status === STATUS_ENDED || status === STATUS_ARCHIVED) return 'ended'
  return undefined
}

/** timeOf 取时刻的毫秒数;读不出来时排在同组最后 */
function timeOf(value: string): number {
  const ms = new Date(value).getTime()
  return Number.isFinite(ms) ? ms : Number.MAX_SAFE_INTEGER
}

/** sortStudentContests 按学生关心的先后排序,返回新数组 */
export function sortStudentContests<T extends OrderContest>(list: readonly T[], joined: ReadonlySet<string>): T[] {
  const rank = (contest: T): number => {
    const segment = contestSegmentOf(contest.status)
    if (segment === 'running') return joined.has(contest.id) ? 0 : 2
    if (segment === 'signup') return 1
    if (segment === 'ended') return 3
    return 4
  }
  return [...list].sort((a, b) => {
    const diff = rank(a) - rank(b)
    if (diff !== 0) return diff
    const segment = contestSegmentOf(a.status)
    if (segment === 'signup') {
      const mine = Number(joined.has(b.id)) - Number(joined.has(a.id))
      return mine !== 0 ? mine : timeOf(a.signup_end) - timeOf(b.signup_end)
    }
    if (segment === 'ended') return timeOf(b.end_at) - timeOf(a.end_at)
    return timeOf(a.end_at) - timeOf(b.end_at)
  })
}

export type TeacherContestSegment = 'draft' | ContestSegment

/** 教师排序用到的赛事字段 */
export interface TeacherOrderContest extends OrderContest {
  start_at: string
}

/** teacherContestSegmentOf 教师列表的分段:草稿单独一段,其余与学生端相同 */
export function teacherContestSegmentOf(status: number): TeacherContestSegment | undefined {
  return status === STATUS_DRAFT ? 'draft' : contestSegmentOf(status)
}

/**
 * sortTeacherContests 按教师接下来要管的先后排序,返回新数组:
 * 进行中(含封榜,先结束的在前)→ 报名中(先开赛的在前)→ 草稿(发布期限即报名截止近的在前)
 * → 已结束待归档 → 已归档(后两组最近结束的在前)。
 */
export function sortTeacherContests<T extends TeacherOrderContest>(list: readonly T[]): T[] {
  const rank = (status: number): number => {
    if (status === STATUS_RUNNING || status === STATUS_FROZEN) return 0
    if (status === STATUS_SIGNUP) return 1
    if (status === STATUS_DRAFT) return 2
    if (status === STATUS_ENDED) return 3
    if (status === STATUS_ARCHIVED) return 4
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
