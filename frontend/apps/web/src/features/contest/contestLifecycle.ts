// contestLifecycle.ts 教师赛事的「下一步」(设计 §7.4 赛事组织 / 赛事详情):草稿→发布、报名中→开赛、
// 进行中→封榜或结束、封榜中→结束、已结束→归档。时间窗口与后端 validateContestTransitionWindow 一致:
// 发布须在报名截止前;开赛须在开赛到结束之间;封榜须在结束前 freeze_minutes 到结束之间;结束须到结束时刻。
// 窗口未到时动作仍然显示,但不可用并说明原因(后台会在开赛、封榜、结束时刻自动推进)。
// 纯函数,供 node:test 直接运行:不得使用 enum,也不得在运行时导入 @chaimir/*。

/** 赛事状态取值,与 @chaimir/api-client 的 ContestStatus 一致 */
const STATUS_DRAFT = 1
const STATUS_SIGNUP = 2
const STATUS_RUNNING = 3
const STATUS_FROZEN = 4
const STATUS_ENDED = 5

export type LifecycleAction = 'publish' | 'start' | 'freeze' | 'end' | 'archive'

/** 判定用到的赛事字段 */
export interface LifecycleContest {
  status: number
  signup_end: string
  start_at: string
  end_at: string
  freeze_minutes: number
}

/** 赛事当前的下一步;available 为 false 时 reason 说明为什么现在还不能做 */
export interface ContestNextStep {
  action: LifecycleAction
  available: boolean
  reason?: string
}

const UNREADABLE_TIME = '赛事时间读不出来,请编辑赛事检查'

/** msOf 取时刻的毫秒数;读不出来时返回 undefined */
function msOf(value: string): number | undefined {
  const ms = new Date(value).getTime()
  return Number.isFinite(ms) ? ms : undefined
}

/** ready 可以立即执行的下一步 */
function ready(action: LifecycleAction): ContestNextStep {
  return { action, available: true }
}

/** waiting 已知下一步但现在还不能做 */
function waiting(action: LifecycleAction, reason: string): ContestNextStep {
  return { action, available: false, reason }
}

/**
 * contestNextStep 按赛事状态与时刻给出「下一步」;已归档(或未知状态)没有下一步。
 * now 只为测试注入。
 */
export function contestNextStep(contest: LifecycleContest, now: Date = new Date()): ContestNextStep | undefined {
  const nowMs = now.getTime()
  switch (contest.status) {
    case STATUS_DRAFT: {
      const signupEnd = msOf(contest.signup_end)
      if (signupEnd === undefined) return waiting('publish', UNREADABLE_TIME)
      return nowMs > signupEnd
        ? waiting('publish', '报名截止时间已过,请先编辑赛事调整时间')
        : ready('publish')
    }
    case STATUS_SIGNUP: {
      const start = msOf(contest.start_at)
      const end = msOf(contest.end_at)
      if (start === undefined || end === undefined) return waiting('start', UNREADABLE_TIME)
      if (nowMs < start) return waiting('start', '到开赛时间会自动开始')
      return nowMs < end ? ready('start') : waiting('start', '已过结束时间,会自动转为已结束')
    }
    case STATUS_RUNNING: {
      const end = msOf(contest.end_at)
      if (end === undefined) return waiting('end', UNREADABLE_TIME)
      if (nowMs >= end) return ready('end')
      if (contest.freeze_minutes > 0) {
        const freezeAt = end - contest.freeze_minutes * 60_000
        return nowMs >= freezeAt ? ready('freeze') : waiting('freeze', '到封榜时间会自动封榜')
      }
      return waiting('end', '到结束时间会自动结束')
    }
    case STATUS_FROZEN: {
      const end = msOf(contest.end_at)
      if (end === undefined) return waiting('end', UNREADABLE_TIME)
      return nowMs >= end ? ready('end') : waiting('end', '到结束时间会自动结束')
    }
    case STATUS_ENDED:
      return ready('archive')
    default:
      return undefined
  }
}
