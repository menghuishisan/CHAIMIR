// 实验实训页(学生侧栏,/student/experiments)。设计 §7.2:
// 页头 + 一行工具条(按课程筛选、搜索);卡片网格,卡片显示实验名、所属课程(用学生自己的课程列表解析)、
// 个人或小组、是否要交报告、阶段数与检查点数;整卡链向实验详情,卡内「进入实验」直接创建或恢复
// 实验环境并进入工作台。课程筛选与搜索写进地址。
//
// 列表走学生专用投影 GET /experiment/student/experiments —— teacher 组的实验读取
// 会返回 init_code_ref/judger_code/extra_input,那是答案链路,学生不可用。
// 「进入实验」调 POST /experiments/{id}/instances:该接口对已有活跃实例幂等(见对齐清单 §6.8),
// 因此不做「开始 / 继续」二态判断,一个按钮既创建也恢复。
// 「个人或小组」「是否交报告」的全量计数来自接口分组计数,只在页头一行属性里说明;接口不接受这两项筛选,
// 用当前页切片筛选会漏掉别的页,故不做成筛选。

import { useCallback, useMemo, useState } from 'react'
import { Link, useNavigate } from 'react-router'
import { FlaskConical, Play, Users } from 'lucide-react'
import { ExperimentCollabMode, PAGINATION_MAX_SIZE, type StudentExperiment } from '@chaimir/api-client'
import {
  Badge,
  Button,
  Callout,
  Card,
  Icon,
  PageHeader,
  PageScaffold,
  Pagination,
  Select,
  Skeleton,
} from '@chaimir/ui'
import { api } from '../../../../app/api'
import { ResourceError } from '../../../../components/ResourceState'
import { SearchBox } from '../../../../components/SearchBox'
import { useAsyncResource, usePagedResource, useUrlState } from '../../../../hooks'
import { facetCount } from '../../../../utils/facets'
import { experimentCollabModeLabel } from '../../../../utils/labels/experiment'
import { userFacingErrorMessage } from '../../../../utils/userFacingError'

/** 地址参数:course 是课程编号(空为全部课程),q 是搜索词 */
const EXPERIMENTS_QUERY = {
  course: { defaultValue: '' },
  q: { defaultValue: '' },
} as const

/** 课程筛选里「全部课程」的取值(Select 不接受空串作选项值) */
const ALL_COURSES = 'all'

/**
 * StudentExperimentsPage 列出可做实验并提供进入入口。
 */
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
  // 学生自己的课程:用于筛选项与卡片上的课程名;一学期几门课,一页取完
  const courses = useAsyncResource(
    () => api.teaching.getCourses({ role: 'student', size: PAGINATION_MAX_SIZE }),
    [],
    () => false,
  )
  const courseName = useMemo(
    () => new Map((courses.data?.list ?? []).map((course) => [course.id, course.name])),
    [courses.data],
  )
  const courseOptions = useMemo(
    () => [
      { value: ALL_COURSES, label: '全部课程' },
      ...(courses.data?.list ?? []).map((course) => ({ value: course.id, label: course.name })),
    ],
    [courses.data],
  )

  // 全量分组计数(与当前筛选同口径),不用当前页切片去数
  const facets = experiments.status === 'success' ? experiments.data?.facets : undefined
  const groupCount = facetCount(facets, 'collab_mode', ExperimentCollabMode.GROUP)
  const reportCount = facetCount(facets, 'require_report', true)

  /**
   * enterExperiment 创建或恢复实例后进入沉浸式工作台。
   * 小组实验必须带 my_group_id:后端按组成员关系校验,未分组时按钮本身不可用(该字段是学生
   * 获取自身 group_id 的唯一来源,见 M7 接口设计 §3.1)。
   */
  const enterExperiment = useCallback(
    async (experiment: StudentExperiment) => {
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
    },
    [navigate],
  )

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
  onEnter: () => void
}

/**
 * ExperimentCard 一个实验:实验名(链向详情,整卡可点)、所属课程、个人或小组、是否交报告、阶段与检查点数;
 * 「进入实验」浮在整卡链接之上单独可点。小组实验未分组时按钮不可用并写明原因。
 */
function ExperimentCard({ experiment, courseName, entering, onEnter }: ExperimentCardProps) {
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
      <div className="relative z-base mt-auto flex flex-wrap items-center gap-3 pt-2">
        <Button
          variant="secondary"
          size="sm"
          leftIcon={Play}
          loading={entering}
          disabled={waitingGroup}
          onClick={onEnter}
        >
          进入实验
        </Button>
        {waitingGroup ? <span className="text-sm text-warning">等待老师分组后才能开始</span> : null}
      </div>
    </Card>
  )
}

/** ExperimentGridSkeleton 与真实卡片同形的骨架 */
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
