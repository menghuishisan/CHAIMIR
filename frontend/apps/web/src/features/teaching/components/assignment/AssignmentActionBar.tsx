// AssignmentActionBar 作答页底部固定操作条(设计 §7.2 作业作答):保存状态(「已保存 · 1 分钟前」)、
// 「已作答 n / m」、「立即保存」、「提交作业」;不能再提交时换成一句原因和「查看结果」。
// 墨色底:它浮在纸面与白色题目卡之上,用墨底与下方的卡片拉开层级。

import { useEffect, useReducer } from 'react'
import { Link } from 'react-router'
import { Lock, Stamp } from 'lucide-react'
import { Button, buttonVariants, Icon, type AutosaveState } from '@chaimir/ui'
import { formatDateTime, formatRelativeOccurrence } from '../../../../utils/formatters'

/** 「N 分钟前」的刷新间隔 */
const RELATIVE_TIME_REFRESH_MS = 30_000

/** 操作条的外观:吸在滚动区底部,与底边留一点距离 */
const BAR_CLASS =
  'sticky bottom-4 z-sticky mt-6 flex flex-wrap items-center justify-between gap-x-4 gap-y-3 rounded-lg bg-dark-surface px-4 py-3 text-sm text-on-dark shadow-lg'

export interface AssignmentActionBarProps {
  autosaveState: AutosaveState
  /** 与服务器上的作答相比有几题改动未保存 */
  dirtyCount: number
  savedAt: Date | undefined
  /** 服务器与本机作答冲突,尚未选定保留哪一份 */
  conflict: boolean
  answered: number
  total: number
  /** 现在提交算迟交 */
  late: boolean
  busy: boolean
  onSave: () => void
  onSubmit: () => void
}

/** AssignmentActionBar 渲染可作答时的操作条 */
export function AssignmentActionBar({
  autosaveState,
  dirtyCount,
  savedAt,
  conflict,
  answered,
  total,
  late,
  busy,
  onSave,
  onSubmit,
}: AssignmentActionBarProps) {
  return (
    <div role="region" aria-label="作答操作" className={BAR_CLASS}>
      <div className="min-w-0">
        <p aria-live="polite" className="tabular-nums">
          <SaveStatus
            state={autosaveState}
            dirtyCount={dirtyCount}
            savedAt={savedAt}
            conflict={conflict}
          />
        </p>
        <p className="text-on-dark-sub tabular-nums">
          已作答 {answered} / {total} 题
          {late ? <span className="text-on-dark-amber"> · 已过截止时间,现在提交算迟交</span> : null}
        </p>
      </div>
      <div className="flex w-full items-center gap-2 sm:w-auto">
        <Button
          variant="on-dark"
          disabled={busy || conflict}
          onClick={onSave}
          className="flex-1 sm:flex-none"
        >
          立即保存
        </Button>
        <Button
          variant="seal"
          leftIcon={Stamp}
          disabled={busy || conflict}
          onClick={onSubmit}
          className="flex-1 sm:flex-none"
        >
          提交作业
        </Button>
      </div>
    </div>
  )
}

/** AssignmentClosedBar 不能再提交时的操作条:一句原因;交过的给「查看结果」 */
export function AssignmentClosedBar({
  reason,
  resultsHref,
}: {
  reason: string
  resultsHref: string | undefined
}) {
  return (
    <div role="region" aria-label="作答操作" className={BAR_CLASS}>
      <p className="flex min-w-0 items-center gap-2">
        <Icon icon={Lock} size="sm" className="shrink-0 text-on-dark-sub" />
        {reason}
      </p>
      {resultsHref ? (
        <Link to={resultsHref} className={buttonVariants({ variant: 'primary' })}>
          查看结果
        </Link>
      ) : null}
    </div>
  )
}

/** SaveStatus 保存状态一句话;「N 分钟前」每 30 秒更新一次 */
function SaveStatus({
  state,
  dirtyCount,
  savedAt,
  conflict,
}: {
  state: AutosaveState
  dirtyCount: number
  savedAt: Date | undefined
  conflict: boolean
}) {
  const [, refresh] = useReducer((count: number) => count + 1, 0)
  useEffect(() => {
    const timer = window.setInterval(refresh, RELATIVE_TIME_REFRESH_MS)
    return () => window.clearInterval(timer)
  }, [])

  if (conflict) return <>先选好保留哪一份作答</>
  if (state === 'saving') return <>正在保存…</>
  if (state === 'error')
    return <span className="text-on-dark-danger">没能保存到服务器,请点「立即保存」重试</span>
  if (dirtyCount > 0) return <>有 {dirtyCount} 题改动未保存</>
  if (savedAt) {
    const iso = savedAt.toISOString()
    return (
      <>
        已保存 ·{' '}
        <time dateTime={iso} title={formatDateTime(iso)}>
          {formatRelativeOccurrence(iso)}
        </time>
      </>
    )
  }
  return <>作答会自动保存到服务器</>
}
