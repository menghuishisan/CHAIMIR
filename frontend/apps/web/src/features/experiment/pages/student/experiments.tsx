import { useMemo, useState } from 'react'
import { Link, useNavigate } from 'react-router'
import { FlaskConical, Play, Users } from 'lucide-react'
import { ExperimentCollabMode, PAGINATION_MAX_SIZE, type StudentExperiment } from '@chaimir/api-client'
import {
  Badge,
  Button,
  Callout,
  Card,
  ChainProgress,
  Icon,
  PageHeader,
  PageScaffold,
  Pagination,
  Select,
  Skeleton,
  StatusIndicator,
} from '@chaimir/ui'
import { api } from '../../../../app/api'
import { collectAllPages } from '../../../../utils/allPages'
import { ResourceError } from '../../../../components/ResourceState'
import { SearchBox } from '../../../../components/SearchBox'
import { useAsyncResource, usePagedResource, useUrlState } from '../../../../hooks'
import { facetCount } from '../../../../utils/facets'
import { formatScore } from '../../../../utils/formatters'
import { experimentCollabModeLabel, experimentInstanceStatusLabel } from '../../../../utils/labels/experiment'
import { experimentInstanceStatusTone } from '../../statusPresentation'
import { userFacingErrorMessage } from '../../../../utils/userFacingError'

const EXPERIMENTS_QUERY = {
  course: { defaultValue: '' },
  q: { defaultValue: '' },
} as const

/** 课程筛选里「全部课程」的取值(Select 不接受空串作选项值) */
const ALL_COURSES = 'all'

export default function StudentExperimentsPage() {
  const navigate = useNavigate()
  const [query, setQuery] = useUrlState(EXPERIMENTS_QUERY)
  const [enteringId, setEnteringId] = useState<string>()
  const [actionError, setActionError] = useState<string>()

  const experiments = usePagedResource<StudentExperiment>(
    (params) =>
      api.experiment.getPublishedExperiments({
        ...params,
        course_id: query.course || undefined,
        keyword: query.q || undefined,
      }),
    [query.course, query.q],
    { urlPageKey: 'page' },
  )
  const courses = useAsyncResource(
    () => collectAllPages((params) => api.teaching.getCourses({ role: 'student', ...params }), PAGINATION_MAX_SIZE),
    [],
    () => false,
  )
  const courseName = useMemo(
    () => new Map((courses.data ?? []).map((course) => [course.id, course.name])),
    [courses.data],
  )
  const courseOptions = useMemo(
    () => [
      { value: ALL_COURSES, label: '全部课程' },
      ...(courses.data ?? []).map((course) => ({ value: course.id, label: course.name })),
    ],
    [courses.data],
  )

  // 分组计数覆盖全部筛选结果，不能从当前页推算。
  const facets = experiments.status === 'success' ? experiments.data?.facets : undefined
  const groupCount = facetCount(facets, 'collab_mode', ExperimentCollabMode.GROUP)
  const reportCount = facetCount(facets, 'require_report', true)

  const enterExperiment = async (experiment: StudentExperiment) => {
    if (enteringId || (experiment.collab_mode === ExperimentCollabMode.GROUP && !experiment.my_group_id)) return
    setEnteringId(experiment.id)
    setActionError(undefined)
    try {
      const instance = await api.experiment.createInstance(experiment.id, {
        group_id: experiment.my_group_id,
      })
      navigate(`/student/experiments/${experiment.id}/workspace?instance=${instance.instance_id}`)
    } catch (error) {
      setActionError(userFacingErrorMessage(error, '实验环境没能准备好,请稍后重试。'))
    } finally {
      setEnteringId(undefined)
    }
  }

  const filtered = query.course !== '' || query.q !== ''
  const list = experiments.data?.list ?? []

  return (
    <PageScaffold>
      <PageHeader
        title="实验实训"
        meta={
          experiments.total !== undefined
            ? [
                { value: `共 ${experiments.total} 个实验` },
                { label: '小组实验', value: groupCount },
                { label: '需交报告', value: reportCount },
              ]
            : undefined
        }
      />

      <div className="mb-5 flex flex-wrap items-center gap-3">
        <Select
          aria-label="按课程筛选"
          value={query.course || ALL_COURSES}
          onValueChange={(value) => setQuery({ course: value === ALL_COURSES ? '' : value })}
          options={courseOptions}
          className="w-full sm:w-64"
        />
        <SearchBox
          label="搜索实验"
          placeholder="搜索实验名称或说明"
          value={query.q}
          onSearch={(value) => setQuery({ q: value })}
        />
      </div>

      {actionError ? (
        <Callout tone="danger" className="mb-4">
          {actionError}
        </Callout>
      ) : null}

      {experiments.status === 'error' ? (
        <ResourceError
          message={experiments.error?.message ?? '实验暂时读取不到'}
          traceId={experiments.error?.traceId}
          onRetry={experiments.reload}
        />
      ) : experiments.status === 'empty' ? (
        <div className="flex flex-col items-center rounded-lg bg-surface px-6 py-14 text-center shadow-xs">
          <Icon icon={FlaskConical} size="lg" className="text-ink-faint" />
          <p className="mt-3 text-md font-semibold text-ink">
            {filtered ? '没有符合条件的实验' : '还没有可以做的实验'}
          </p>
          <p className="mt-1.5 text-sm text-ink-sub">
            {filtered ? '换个课程或关键词试试。' : '老师发布实验后会显示在这里,可以直接进入实验环境动手做。'}
          </p>
          {filtered ? (
            <Button variant="outline" className="mt-5" onClick={() => setQuery({ course: '', q: '' })}>
              看全部实验
            </Button>
          ) : null}
        </div>
      ) : experiments.status === 'loading' && list.length === 0 ? (
        <ExperimentGridSkeleton />
      ) : (
        <>
          <ul className="card-grid gap-4" aria-busy={experiments.status === 'loading' || undefined}>
            {list.map((experiment) => (
              <li key={experiment.id}>
                <ExperimentCard
                  experiment={experiment}
                  courseName={experiment.course_id ? courseName.get(experiment.course_id) : undefined}
                  entering={enteringId === experiment.id}
                  blocked={enteringId !== undefined}
                  onEnter={() => void enterExperiment(experiment)}
                />
              </li>
            ))}
          </ul>
          <div className="mt-6">
            <Pagination
              page={experiments.page}
              pageSize={experiments.pageSize}
              total={experiments.total}
              onPageChange={experiments.setPage}
            />
          </div>
        </>
      )}
    </PageScaffold>
  )
}

interface ExperimentCardProps {
  experiment: StudentExperiment
  courseName: string | undefined
  entering: boolean
  blocked: boolean
  onEnter: () => void
}

function ExperimentCard({ experiment, courseName, entering, blocked, onEnter }: ExperimentCardProps) {
  const group = experiment.collab_mode === ExperimentCollabMode.GROUP
  const waitingGroup = group && !experiment.my_group_id
  const { stages, checkpoints } = experiment.components
  return (
    <Card interactive className="flex h-full flex-col gap-3 p-5">
      <div className="flex items-start justify-between gap-3">
        <Link
          to={`/student/experiments/${experiment.id}`}
          className="card-link line-clamp-2 text-lg font-bold text-ink"
        >
          {experiment.name}
        </Link>
        <Badge tone={group ? 'info' : 'neutral'} className="shrink-0">
          {group ? <Icon icon={Users} size="xs" /> : null}
          {experimentCollabModeLabel(experiment.collab_mode)}
        </Badge>
      </div>
      {courseName ? <p className="truncate text-sm text-ink-sub">{courseName}</p> : null}
      {experiment.description ? (
        <p className="line-clamp-2 text-sm text-ink-sub">{experiment.description}</p>
      ) : null}
      <p className="text-sm text-ink-sub">
        {stages.length} 个阶段 · {checkpoints.length} 个检查点 · {experiment.require_report ? '需交报告' : '不用交报告'}
      </p>
      <ExperimentProgressLine progress={experiment.my_progress} checkpointCount={checkpoints.length} />
      <div className="relative z-base mt-auto flex flex-wrap items-center gap-3 pt-2">
        <Button
          variant="secondary"
          size="sm"
          leftIcon={Play}
          loading={entering}
          disabled={waitingGroup || (blocked && !entering)}
          onClick={onEnter}
        >
          进入实验
        </Button>
        {waitingGroup ? <span className="text-sm text-warning">等待老师分组后才能开始</span> : null}
      </div>
    </Card>
  )
}

function ExperimentProgressLine({
  progress,
  checkpointCount,
}: {
  progress: StudentExperiment['my_progress']
  checkpointCount: number
}) {
  if (!progress) return <p className="text-sm text-ink-sub">还没开始</p>
  return (
    <div className="flex flex-wrap items-center gap-x-3 gap-y-2">
      <StatusIndicator tone={experimentInstanceStatusTone(progress.status)} label={experimentInstanceStatusLabel(progress.status)} />
      {checkpointCount > 0 ? (
        <ChainProgress size="sm" label="已通过的检查点" total={checkpointCount} done={Math.min(progress.passed_checkpoints, checkpointCount)} />
      ) : null}
      <span className="text-sm tabular-nums text-ink-sub">得分 {formatScore(progress.score)}</span>
    </div>
  )
}

function ExperimentGridSkeleton() {
  return (
    <ul className="card-grid gap-4" aria-busy="true" aria-label="正在读取实验">
      {Array.from({ length: 3 }, (_, index) => (
        <li key={index} className="flex flex-col gap-3 rounded-lg bg-surface p-5 shadow-xs">
          <Skeleton variant="title" width="half" />
          <Skeleton variant="line" lines={2} />
          <Skeleton variant="line" width="third" />
        </li>
      ))}
    </ul>
  )
}
