// contestTimeline.ts 计算赛程条与卡片时间提示(设计 §7.2 竞赛列表 / 竞赛详情 / 教师赛事详情)。
// 赛程节点:报名开始 → 报名截止 → 开赛 → 封榜(结束前 freeze_minutes;不封榜时没有)→ 结束。
// 当前阶段以赛事状态为准:状态由服务端定时或教师手动推进,不一定与时刻表对齐;
// 只在「报名中」状态下用时刻区分「还在报名」与「报名已截止、等待开赛」。
// 纯函数,供 node:test 直接运行:不得使用 enum,也不得在运行时导入 @chaimir/*。

/** 赛事状态取值,与 @chaimir/api-client 的 ContestStatus 一致 */
const STATUS_DRAFT = 1
const STATUS_SIGNUP = 2
const STATUS_RUNNING = 3
const STATUS_FROZEN = 4
const STATUS_ENDED = 5
const STATUS_ARCHIVED = 6

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

/** currentKey 按状态(报名中再看时刻)给出当前所在节点;草稿等未公开状态没有当前节点 */
function currentKey(contest: TimelineContest, now: Date, hasFreeze: boolean): TimelineNodeKey | undefined {
  switch (contest.status) {
    case STATUS_SIGNUP: {
      const signupEnd = new Date(contest.signup_end).getTime()
      return Number.isFinite(signupEnd) && now.getTime() > signupEnd ? 'signup_end' : 'signup_start'
    }
    case STATUS_RUNNING:
      return 'start'
    case STATUS_FROZEN:
      return hasFreeze ? 'freeze' : 'start'
    case STATUS_ENDED:
    case STATUS_ARCHIVED:
      return 'end'
    default:
      return undefined
  }
}

/** contestTimeline 给出赛程节点及各自状态;now 只为测试注入 */
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
    case STATUS_DRAFT: {
      const signupEnd = ms(contest.signup_end)
      if (signupEnd === undefined) return undefined
      return nowMs <= signupEnd ? `${durationText(now, new Date(signupEnd))}内须发布` : '报名截止时间已过'
    }
    case STATUS_SIGNUP: {
      const signupEnd = ms(contest.signup_end)
      if (signupEnd === undefined) return undefined
      if (nowMs <= signupEnd) return `报名还剩 ${durationText(now, new Date(signupEnd))}`
      const start = ms(contest.start_at)
      if (start === undefined) return '报名已截止'
      return nowMs < start ? `报名已截止,${durationText(now, new Date(start))}后开赛` : '报名已截止,等待开赛'
    }
    case STATUS_RUNNING:
    case STATUS_FROZEN: {
      const end = ms(contest.end_at)
      if (end === undefined) return undefined
      return nowMs < end ? `距结束 ${durationText(now, new Date(end))}` : '即将结束'
    }
    case STATUS_ENDED:
    case STATUS_ARCHIVED: {
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
