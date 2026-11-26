import { ContestStatus } from '@chaimir/api-client'

/** 计算赛程用到的赛事字段 */
export interface TimelineContest {
  status: number
  signup_start: string
  signup_end: string
  start_at: string
  end_at: string
  freeze_minutes: number
}

export type TimelineNodeKey = 'signup_start' | 'signup_end' | 'start' | 'freeze' | 'end'

/** 赛程条上的一个节点 */
export interface TimelineNode {
  key: TimelineNodeKey
  label: string
  /** 节点时刻(ISO 字符串) */
  at: string
  state: 'done' | 'current' | 'upcoming'
}

const NODE_LABELS: Record<TimelineNodeKey, string> = {
  signup_start: '报名开始',
  signup_end: '报名截止',
  start: '开赛',
  freeze: '封榜',
  end: '结束',
}

/** 报名窗口与服务端一致，包含起止时刻；其它赛事状态不可报名。 */
export function contestSignupState(
  contest: Pick<TimelineContest, 'status' | 'signup_start' | 'signup_end'>,
  now: number,
): 'upcoming' | 'open' | 'closed' {
  if (contest.status !== ContestStatus.SIGNUP) return 'closed'
  const start = Date.parse(contest.signup_start)
  const end = Date.parse(contest.signup_end)
  if (!Number.isFinite(start) || !Number.isFinite(end) || start > end || !Number.isFinite(now)) return 'closed'
  return now < start ? 'upcoming' : now <= end ? 'open' : 'closed'
}

// 阶段以服务端状态为准；报名阶段再按窗口区分未开始、开放与截止。
function currentKey(contest: TimelineContest, now: Date, hasFreeze: boolean): TimelineNodeKey | undefined {
  switch (contest.status) {
    case ContestStatus.SIGNUP: {
      const signup = contestSignupState(contest, now.getTime())
      return signup === 'upcoming' ? undefined : signup === 'open' ? 'signup_start' : 'signup_end'
    }
    case ContestStatus.RUNNING:
      return 'start'
    case ContestStatus.FROZEN:
      return hasFreeze ? 'freeze' : 'start'
    case ContestStatus.ENDED:
    case ContestStatus.ARCHIVED:
      return 'end'
    default:
      return undefined
  }
}

/** contestTimeline 给出赛程节点及各自状态。 */
export function contestTimeline(contest: TimelineContest, now: Date = new Date()): TimelineNode[] {
  const hasFreeze = contest.freeze_minutes > 0
  const keys: TimelineNodeKey[] = hasFreeze
    ? ['signup_start', 'signup_end', 'start', 'freeze', 'end']
    : ['signup_start', 'signup_end', 'start', 'end']
  const endMs = new Date(contest.end_at).getTime()
  const freezeAt = Number.isFinite(endMs)
    ? new Date(endMs - contest.freeze_minutes * 60_000).toISOString()
    : contest.end_at
  const at: Record<TimelineNodeKey, string> = {
    signup_start: contest.signup_start,
    signup_end: contest.signup_end,
    start: contest.start_at,
    freeze: freezeAt,
    end: contest.end_at,
  }
  const current = currentKey(contest, now, hasFreeze)
  const currentIndex = current === undefined ? -1 : keys.indexOf(current)
  return keys.map((key, index) => ({
    key,
    label: NODE_LABELS[key],
    at: at[key],
    state: currentIndex < 0 || index > currentIndex ? 'upcoming' : index === currentIndex ? 'current' : 'done',
  }))
}

/**
 * durationText 把 from 到 to 的一段未来时长写成「N 分钟 / N 小时 / N 天」:
 * 不足 24 小时按小时(不足 1 小时按分钟),否则按日历天数 —— 与作业截止「还剩 N 天」同一口径。
 */
function durationText(from: Date, to: Date): string {
  const ms = to.getTime() - from.getTime()
  const minutes = Math.max(1, Math.ceil(ms / 60_000))
  if (minutes < 60) return `${minutes} 分钟`
  if (ms < 86_400_000) return `${Math.floor(minutes / 60)} 小时`
  const fromDay = new Date(from.getFullYear(), from.getMonth(), from.getDate()).getTime()
  const toDay = new Date(to.getFullYear(), to.getMonth(), to.getDate()).getTime()
  return `${Math.round((toDay - fromDay) / 86_400_000)} 天`
}

/** dotDate 把日期写成 2026.11.15 */
function dotDate(date: Date): string {
  const pad = (value: number) => String(value).padStart(2, '0')
  return `${date.getFullYear()}.${pad(date.getMonth() + 1)}.${pad(date.getDate())}`
}

/**
 * contestTimeHint 给出列表上的时间提示:草稿「2 天内须发布」(发布须在报名截止前,过了写「报名截止时间已过」,
 * 只在教师端出现),报名中「报名还剩 2 天」(截止后「报名已截止,2 天后开赛」),
 * 进行中或封榜「距结束 3 小时」,已结束或已归档「2026.11.15 结束」。时刻读不出来时返回 undefined。
 */
export function contestTimeHint(contest: TimelineContest, now: Date = new Date()): string | undefined {
  const nowMs = now.getTime()
  const ms = (value: string) => {
    const parsed = new Date(value).getTime()
    return Number.isFinite(parsed) ? parsed : undefined
  }
  switch (contest.status) {
    case ContestStatus.DRAFT: {
      const signupEnd = ms(contest.signup_end)
      if (signupEnd === undefined) return undefined
      return nowMs <= signupEnd ? `${durationText(now, new Date(signupEnd))}内须发布` : '报名截止时间已过'
    }
    case ContestStatus.SIGNUP: {
      const signupEnd = ms(contest.signup_end)
      if (signupEnd === undefined) return undefined
      if (contestSignupState(contest, nowMs) === 'upcoming') return `${durationText(now, new Date(contest.signup_start))}后开始报名`
      if (nowMs <= signupEnd) return `报名还剩 ${durationText(now, new Date(signupEnd))}`
      const start = ms(contest.start_at)
      if (start === undefined) return '报名已截止'
      return nowMs < start ? `报名已截止,${durationText(now, new Date(start))}后开赛` : '报名已截止,等待开赛'
    }
    case ContestStatus.RUNNING:
    case ContestStatus.FROZEN: {
      const end = ms(contest.end_at)
      if (end === undefined) return undefined
      return nowMs < end ? `距结束 ${durationText(now, new Date(end))}` : '即将结束'
    }
    case ContestStatus.ENDED:
    case ContestStatus.ARCHIVED: {
      const end = ms(contest.end_at)
      return end === undefined ? undefined : `${dotDate(new Date(end))} 结束`
    }
    default:
      return undefined
  }
}

/** 答题倒计时进入紧迫的阈值:最后 10 分钟(设计 §7.2 竞赛答题) */
const COUNTDOWN_URGENT_MS = 10 * 60_000

/**
 * contestCountdown 答题工作台顶栏的剩余时间:写成「时:分:秒」,最后 10 分钟 urgent,
 * 已到结束时刻 over 且显示 0:00:00。结束时刻读不出来时返回 undefined(不显示倒计时)。
 */
export function contestCountdown(
  endAt: string,
  now: Date = new Date(),
): { text: string; urgent: boolean; over: boolean } | undefined {
  const end = new Date(endAt).getTime()
  if (!Number.isFinite(end)) return undefined
  const left = Math.max(0, end - now.getTime())
  const totalSeconds = Math.floor(left / 1000)
  const hours = Math.floor(totalSeconds / 3600)
  const minutes = Math.floor((totalSeconds % 3600) / 60)
  const seconds = totalSeconds % 60
  const pad = (value: number) => String(value).padStart(2, '0')
  return {
    text: `${hours}:${pad(minutes)}:${pad(seconds)}`,
    urgent: left <= COUNTDOWN_URGENT_MS,
    over: left === 0,
  }
}
