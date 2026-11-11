// 学业预警页(学生侧栏,/student/alerts)。设计 §7.2:
// 待确认的预警排在最前,每条一张白卡(学期、预警类型与说明、建议下一步、「我已知悉」);
// 已确认的折叠在下方「已确认(n)」。「收到预警怎么办」三步说明放在有待确认预警时的卡片内,
// 给「打开成绩中心」。没有预警时用积极的话:「目前没有学业预警」。分页只在超过一页时出现。
//
// GET /grade-center/warnings 的 student_id 缺省即本人(后端在缺省时填会话账号,并校验学生只能读自己),
// 前端不传该参数。预警明细 detail 是后端按类型写入的开放对象:只呈现已登记的键,未登记键不猜语义。
// 一个学生的预警通常 0–3 条:待确认与已确认分两次按状态读取,各自分页。

import { useCallback, useMemo, useState } from 'react'
import { Link } from 'react-router'
import { CheckCheck, ChevronDown, GraduationCap, ShieldCheck, TriangleAlert } from 'lucide-react'
import { GradeWarningStatus, GradeWarningType, type GradeWarning } from '@chaimir/api-client'
import { Button, buttonVariants, Callout, cn, Icon, PageHeader, PageScaffold, Pagination, Skeleton, toast } from '@chaimir/ui'
import { api } from '../../../../app/api'
import { ResourceError } from '../../../../components/ResourceState'
import { useAsyncResource, usePagedResource } from '../../../../hooks'
import { formatRelativeOccurrence, formatDateTime } from '../../../../utils/formatters'
import { gradeWarningDetailTerm, gradeWarningTypeLabel } from '../../../../utils/labels/grade'
import { userFacingErrorMessage } from '../../../../utils/userFacingError'

/**
 * StudentAlertsPage 列出本人学业预警并提供确认。
 */
export default function StudentAlertsPage() {
  const [ackingId, setAckingId] = useState<string>()
  const [actionError, setActionError] = useState<string>()
  const [showAcknowledged, setShowAcknowledged] = useState(false)

  const pending = usePagedResource<GradeWarning>(
    (params) => api.grade.listWarnings({ status: GradeWarningStatus.PENDING, ...params }),
    [],
    { urlPageKey: 'page' },
  )
  const acknowledged = usePagedResource<GradeWarning>(
    (params) => api.grade.listWarnings({ status: GradeWarningStatus.ACKNOWLEDGED, ...params }),
    [],
  )
  const semesters = useAsyncResource(() => api.grade.listSemesters(), [], () => false)
  const semesterName = useMemo(
    () => new Map((semesters.data ?? []).map((semester) => [semester.id, semester.name])),
    [semesters.data],
  )

  /** ackWarning 确认预警:确认表示学生已知悉,不改变成绩本身 */
  const ackWarning = useCallback(
    async (warning: GradeWarning) => {
      setAckingId(warning.id)
      setActionError(undefined)
      try {
        await api.grade.ackWarning(warning.id)
        toast.success('已确认这条预警')
        pending.reload()
        acknowledged.reload()
      } catch (error) {
        setActionError(userFacingErrorMessage(error, '确认没有成功,请稍后重试。'))
      } finally {
        setAckingId(undefined)
      }
    },
    [acknowledged, pending],
  )

  const loading = pending.status === 'loading' && !pending.data
  const pendingList = pending.data?.list ?? []
  const acknowledgedTotal = acknowledged.total ?? 0
  const nothing = pending.status === 'empty' && acknowledged.status === 'empty'

  return (
    <PageScaffold>
      <div className="max-w-3xl">
        <PageHeader
          title="学业预警"
          meta={pending.total !== undefined && pending.total > 0 ? [{ value: `${pending.total} 条待确认` }] : undefined}
        />

        {actionError ? (
          <Callout tone="danger" className="mb-4">
            {actionError}
          </Callout>
        ) : null}

        {pending.status === 'error' ? (
          <ResourceError
            message={pending.error?.message ?? '预警暂时读取不到'}
            traceId={pending.error?.traceId}
            onRetry={pending.reload}
          />
        ) : loading ? (
          <div aria-busy="true" className="flex flex-col gap-4">
            <Skeleton variant="block" />
          </div>
        ) : nothing ? (
          <div className="flex flex-col items-center rounded-lg bg-surface px-6 py-14 text-center shadow-xs">
            <Icon icon={ShieldCheck} size="lg" className="text-primary" />
            <p className="mt-3 text-md font-semibold text-ink">目前没有学业预警</p>
            <p className="mt-1.5 text-sm text-ink-sub">继续保持。学校发出预警时会显示在这里,并通过站内通知告诉你。</p>
          </div>
        ) : (
          <div className="flex flex-col gap-4">
            {pendingList.length > 0 ? (
              <>
                <GuidanceCard />
                {pendingList.map((warning) => (
                  <WarningCard
                    key={warning.id}
                    warning={warning}
                    semester={semesterName.get(warning.semester_id)}
                    acking={ackingId === warning.id}
                    onAck={() => void ackWarning(warning)}
                  />
                ))}
                <Pagination page={pending.page} pageSize={pending.pageSize} total={pending.total} onPageChange={pending.setPage} />
              </>
            ) : (
              <p className="flex items-center gap-2 text-sm text-ink-sub">
                <Icon icon={ShieldCheck} size="sm" className="text-primary" />
                没有待确认的预警。
              </p>
            )}

            {acknowledgedTotal > 0 ? (
              <section aria-labelledby="acknowledged-title" className="mt-4">
                <button
                  type="button"
                  id="acknowledged-title"
                  aria-expanded={showAcknowledged}
                  onClick={() => setShowAcknowledged((value) => !value)}
                  className="flex items-center gap-1.5 rounded-md text-sm font-medium text-ink-sub hover:text-ink focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-accent"
                >
                  <Icon icon={ChevronDown} size="sm" className={cn('transition-transform duration-fast', showAcknowledged ? '' : '-rotate-90')} />
                  已确认({acknowledgedTotal})
                </button>
                {showAcknowledged ? (
                  <div className="mt-3 flex flex-col gap-3">
                    {(acknowledged.data?.list ?? []).map((warning) => (
                      <WarningCard key={warning.id} warning={warning} semester={semesterName.get(warning.semester_id)} />
                    ))}
                    <Pagination
                      page={acknowledged.page}
                      pageSize={acknowledged.pageSize}
                      total={acknowledged.total}
                      onPageChange={acknowledged.setPage}
                    />
                  </div>
                ) : null}
              </section>
            ) : null}
          </div>
        )}
      </div>
    </PageScaffold>
  )
}

/** nextStepFor 按预警类型给出一句建议 */
function nextStepFor(warning: GradeWarning): string {
  return warning.type === GradeWarningType.FAILED_COURSE
    ? '尽快联系任课老师,了解补考或重修安排。'
    : '看看哪几门课拉低了绩点,和辅导员或任课老师一起定个补救计划。'
}

/** WarningCard 一条预警:类型、学期与发出时间、明细、建议下一步;待确认的带「我已知悉」 */
function WarningCard({
  warning,
  semester,
  acking,
  onAck,
}: {
  warning: GradeWarning
  semester: string | undefined
  acking?: boolean
  onAck?: () => void
}) {
  const pendingAck = onAck !== undefined
  const details = Object.entries(warning.detail).flatMap(([key, value]) => {
    const term = gradeWarningDetailTerm(key)
    if (!term || (typeof value !== 'string' && typeof value !== 'number')) return []
    return [`${term} ${value}`]
  })
  return (
    <article className={cn('rounded-lg bg-surface p-5 shadow-xs', !pendingAck && 'opacity-80')}>
      <div className="flex flex-wrap items-start justify-between gap-3">
        <div className="min-w-0">
          <h2 className="flex items-center gap-2 text-md font-semibold text-ink">
            {pendingAck ? <Icon icon={TriangleAlert} size="sm" className="text-warning" /> : null}
            {gradeWarningTypeLabel(warning.type)}
          </h2>
          <p className="mt-0.5 text-sm text-ink-sub">
            {semester ? `${semester} · ` : ''}
            <time dateTime={warning.created_at} title={formatDateTime(warning.created_at)}>
              {formatRelativeOccurrence(warning.created_at)}
            </time>
            发出
          </p>
        </div>
        {pendingAck ? (
          <Button variant="primary" size="sm" leftIcon={CheckCheck} loading={acking} onClick={onAck}>
            我已知悉
          </Button>
        ) : (
          <span className="text-sm text-ink-sub">已确认</span>
        )}
      </div>
      {details.length > 0 ? <p className="mt-3 text-sm text-ink">{details.join(' · ')}</p> : null}
      {pendingAck ? <p className="mt-2 text-sm text-ink-sub">建议:{nextStepFor(warning)}</p> : null}
    </article>
  )
}

/** GuidanceCard 收到预警怎么办:三步说明 + 打开成绩中心;说清「我已知悉」不改变成绩 */
function GuidanceCard() {
  return (
    <section aria-labelledby="alert-guidance-title" className="rounded-lg bg-warning-bg p-5">
      <h2 id="alert-guidance-title" className="text-md font-semibold text-ink">
        收到预警怎么办
      </h2>
      <ol className="mt-3 flex list-decimal flex-col gap-1.5 pl-5 text-sm text-ink">
        <li>在成绩中心确认是哪几门课程分数偏低;</li>
        <li>联系任课老师了解补救方式(补考、重修或补交作业);</li>
        <li>对成绩有疑问时,在成绩中心对那门课提出申诉。</li>
      </ol>
      <p className="mt-3 text-sm text-ink-sub">点「我已知悉」只表示你看到了这条提醒,不会改变成绩,也不代表问题已经解决。</p>
      <Link to="/student/grades" className={`${buttonVariants({ variant: 'secondary', size: 'sm' })} mt-4`}>
        <Icon icon={GraduationCap} size="sm" />
        打开成绩中心
      </Link>
    </section>
  )
}
