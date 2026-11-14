// 实验编排页(教师侧栏,/teacher/experiments)。设计 §7.4:
// 页头 +「新建实验」;分段「全部 / 草稿 / 已发布 / 已下架」+ 搜索(都写进地址);表格(名称、所属课程、
// 方式、状态、更新时间),整行链向实验向导;行内按状态给一个快捷按钮:草稿为「校验并发布」,
// 已发布为「报告与小组」;下架放进更多菜单。校验不通过时弹窗逐条列出问题,通过后确认发布。
// 实验定义是多步骤配置(wizard_step 落服务端,刷新不丢,FE-7),编排在向导页完成。

import { useCallback, useMemo, useState } from 'react'
import { Link, useNavigate } from 'react-router'
import { CircleCheck, ClipboardCheck, LayoutTemplate, MoreVertical, Pencil, Plus, Undo2 } from 'lucide-react'
import { ExperimentStatus, PAGINATION_MAX_SIZE, type Experiment, type ValidationResult } from '@chaimir/api-client'
import {
  Button,
  buttonVariants,
  Callout,
  Icon,
  IconButton,
  Menu,
  MenuContent,
  MenuItem,
  MenuTrigger,
  Modal,
  ModalBody,
  ModalContent,
  ModalDescription,
  ModalFooter,
  ModalHeader,
  ModalTitle,
  PageHeader,
  PageScaffold,
  Pagination,
  SegmentedControl,
  Table,
  toast,
  type TableColumn,
} from '@chaimir/ui'
import { api } from '../../../../app/api'
import { collectAllPages } from '../../../../utils/allPages'
import { ResourceError } from '../../../../components/ResourceState'
import { SearchBox } from '../../../../components/SearchBox'
import { useAsyncResource, usePagedResource, useUrlState } from '../../../../hooks'
import { formatDateTime, formatRelativeOccurrence } from '../../../../utils/formatters'
import { experimentCollabModeLabel } from '../../../../utils/labels/experiment'
import { userFacingErrorMessage } from '../../../../utils/userFacingError'
import { ExperimentStatusCell } from '../../components/ExperimentTableCells'
import { ExperimentValidationIssues } from '../../components/ExperimentValidationIssues'

/** 地址参数:status 实验状态分段,q 搜索词 */
const EXPERIMENTS_QUERY = {
  status: { defaultValue: 'all', allowed: ['all', 'draft', 'published', 'unpublished'] },
  q: { defaultValue: '' },
} as const

const STATUS_SEGMENTS = [
  { value: 'all', label: '全部' },
  { value: 'draft', label: '草稿' },
  { value: 'published', label: '已发布' },
  { value: 'unpublished', label: '已下架' },
] as const

/** statusOf 分段对应的实验状态 */
function statusOf(segment: string): ExperimentStatus | undefined {
  if (segment === 'draft') return ExperimentStatus.DRAFT
  if (segment === 'published') return ExperimentStatus.PUBLISHED
  if (segment === 'unpublished') return ExperimentStatus.UNPUBLISHED
  return undefined
}

/**
 * TeacherExperimentsPage 列出实验定义并承载发布与下架。
 */
export default function TeacherExperimentsPage() {
  const navigate = useNavigate()
  const [query, setQuery] = useUrlState(EXPERIMENTS_QUERY)
  const [validateResult, setValidateResult] = useState<{ experiment: Experiment; result: ValidationResult }>()
  const [publishTarget, setPublishTarget] = useState<Experiment>()
  const [working, setWorking] = useState(false)
  const [actionError, setActionError] = useState<string>()

  const experiments = usePagedResource<Experiment>(
    (params) => api.experiment.getExperiments({ status: statusOf(query.status), keyword: query.q || undefined, ...params }),
    [query.q, query.status],
    { urlPageKey: 'page' },
  )
  // 所属课程名:用教师自己的课程列表解析(一页取完)
  const courses = useAsyncResource(() => collectAllPages((params) => api.teaching.getCourses({ role: 'teacher', ...params }), PAGINATION_MAX_SIZE), [], () => false)
  const courseName = useMemo(() => new Map((courses.data ?? []).map((course) => [course.id, course.name])), [courses.data])

  /** validateExperiment 发布前校验:把问题列清楚,而不是让发布直接失败。 */
  const validateExperiment = useCallback(async (experiment: Experiment) => {
    setWorking(true)
    setActionError(undefined)
    try {
      const result = await api.experiment.validateExperiment(experiment.id)
      setValidateResult({ experiment, result })
    } catch (error) {
      setActionError(userFacingErrorMessage(error, '校验没有完成,请稍后重试。'))
    } finally {
      setWorking(false)
    }
  }, [])

  /** publishExperiment 发布实验:发布后学生可见并可创建实例。 */
  const publishExperiment = useCallback(async () => {
    if (!publishTarget) return
    setWorking(true)
    setActionError(undefined)
    try {
      await api.experiment.publishExperiment(publishTarget.id)
      toast.success('实验已发布')
      setPublishTarget(undefined)
      setValidateResult(undefined)
      experiments.reload()
    } catch (error) {
      setActionError(userFacingErrorMessage(error, '发布没有成功,请稍后重试。'))
    } finally {
      setWorking(false)
    }
  }, [experiments, publishTarget])

  /** unpublishExperiment 下架实验:已有实例不受影响,但学生不能再新建。 */
  const unpublishExperiment = useCallback(
    async (experiment: Experiment) => {
      setActionError(undefined)
      try {
        await api.experiment.unpublishExperiment(experiment.id)
        toast.success('实验已下架')
        experiments.reload()
      } catch (error) {
        setActionError(userFacingErrorMessage(error, '下架没有成功,请稍后重试。'))
      }
    },
    [experiments]
  )

  const columns: TableColumn<Experiment>[] = [
    {
      key: 'name',
      header: '实验',
      primary: true,
      render: (experiment) => <span className="font-medium text-ink">{experiment.name}</span>,
    },
    {
      key: 'course_id',
      header: '所属课程',
      render: (experiment) => (
        <span className="text-ink-sub">{experiment.course_id ? (courseName.get(experiment.course_id) ?? '其他课程') : '不属于课程'}</span>
      ),
    },
    {
      key: 'collab_mode',
      header: '方式',
      render: (experiment) => <span className="text-ink-sub">{experimentCollabModeLabel(experiment.collab_mode)}</span>,
    },
    { key: 'status', header: '状态', render: (experiment) => <ExperimentStatusCell experiment={experiment} /> },
    {
      key: 'updated_at',
      header: '更新',
      render: (experiment) => (
        <time dateTime={experiment.updated_at} title={formatDateTime(experiment.updated_at)} className="whitespace-nowrap text-sm text-ink-sub">
          {formatRelativeOccurrence(experiment.updated_at)}
        </time>
      ),
    },
    {
      key: 'actions',
      header: <span className="sr-only">操作</span>,
      align: 'right',
      render: (experiment) => (
        <div className="flex items-center justify-end gap-1">
          {experiment.status === ExperimentStatus.PUBLISHED ? (
            <Button
              variant="secondary"
              size="sm"
              leftIcon={ClipboardCheck}
              onClick={() => navigate(`/teacher/experiments/${experiment.id}/reports`)}
            >
              报告与小组
            </Button>
          ) : experiment.status === ExperimentStatus.DRAFT ? (
            <Button variant="secondary" size="sm" leftIcon={CircleCheck} loading={working} onClick={() => void validateExperiment(experiment)}>
              校验并发布
            </Button>
          ) : null}
          <Menu>
            <MenuTrigger asChild>
              <IconButton variant="ghost" size="sm" icon={MoreVertical} aria-label={`${experiment.name} 的更多操作`} />
            </MenuTrigger>
            <MenuContent align="end">
              <MenuItem icon={Pencil} onSelect={() => navigate(`/teacher/experiments/${experiment.id}`)}>
                打开编排
              </MenuItem>
              {experiment.status !== ExperimentStatus.PUBLISHED ? (
                <MenuItem icon={ClipboardCheck} onSelect={() => navigate(`/teacher/experiments/${experiment.id}/reports`)}>
                  报告与小组
                </MenuItem>
              ) : (
                <MenuItem icon={Undo2} onSelect={() => void unpublishExperiment(experiment)}>
                  下架实验
                </MenuItem>
              )}
              {experiment.status === ExperimentStatus.UNPUBLISHED ? (
                <MenuItem icon={CircleCheck} onSelect={() => void validateExperiment(experiment)}>
                  校验并重新发布
                </MenuItem>
              ) : null}
            </MenuContent>
          </Menu>
        </div>
      ),
    },
  ]

  const list = experiments.data?.list ?? []
  const filtered = query.status !== 'all' || query.q !== ''

  return (
    <PageScaffold>
      <PageHeader
        title="实验编排"
        actions={
          <Link to="/teacher/experiments/new" className={buttonVariants({ variant: 'primary' })}>
            <Icon icon={Plus} size="sm" />
            新建实验
          </Link>
        }
      />

      <div className="mb-5 flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
        <SegmentedControl
          aria-label="按实验状态筛选"
          options={STATUS_SEGMENTS.map((segment) => ({ value: segment.value, label: segment.label }))}
          value={query.status}
          onValueChange={(value) => setQuery({ status: value })}
        />
        <SearchBox label="搜索实验" placeholder="搜索实验名称或说明" value={query.q} onSearch={(value) => setQuery({ q: value })} />
      </div>

      {actionError ? (
        <Callout tone="danger" className="mb-4">
          {actionError}
        </Callout>
      ) : null}

      {experiments.status === 'error' ? (
        <ResourceError message={experiments.error?.message ?? '实验暂时读取不到'} traceId={experiments.error?.traceId} onRetry={experiments.reload} />
      ) : experiments.status === 'empty' ? (
        <div className="flex flex-col items-center rounded-lg bg-surface px-6 py-14 text-center shadow-xs">
          <Icon icon={LayoutTemplate} size="lg" className="text-ink-faint" />
          <p className="mt-3 text-md font-semibold text-ink">{filtered ? '没有符合条件的实验' : '还没有实验'}</p>
          <p className="mt-1.5 text-sm text-ink-sub">
            {filtered ? '换个状态或关键词试试。' : '新建实验后配置环境与检查点,校验通过即可发布给学生。'}
          </p>
          {filtered ? (
            <Button variant="outline" className="mt-5" onClick={() => setQuery({ status: 'all', q: '' })}>
              看全部实验
            </Button>
          ) : (
            <Link to="/teacher/experiments/new" className={`${buttonVariants({ variant: 'primary' })} mt-5`}>
              新建实验
            </Link>
          )}
        </div>
      ) : (
        <>
          <Table
            columns={columns}
            data={list}
            rowKey={(item) => item.id}
            loading={experiments.status === 'loading' && list.length === 0}
            getRowHref={(item) => `/teacher/experiments/${item.id}`}
            onNavigate={(href) => navigate(href)}
            mobileCard={(item) => ({
              title: item.name,
              meta: `${item.course_id ? (courseName.get(item.course_id) ?? '其他课程') : '不属于课程'} · ${experimentCollabModeLabel(item.collab_mode)} · ${formatRelativeOccurrence(item.updated_at)}`,
              badge: <ExperimentStatusCell experiment={item} />,
            })}
          />
          <div className="mt-4">
            <Pagination page={experiments.page} pageSize={experiments.pageSize} total={experiments.total} onPageChange={experiments.setPage} />
          </div>
        </>
      )}

      <Modal
        open={validateResult !== undefined}
        onOpenChange={(open) => !open && setValidateResult(undefined)}
      >
        <ModalContent size="lg">
          <ModalHeader>
            <ModalTitle>发布前校验结果</ModalTitle>
            <ModalDescription>
              校验会检查运行时是否可用、题目版本是否锁定、检查点分值是否合理。
            </ModalDescription>
          </ModalHeader>
          <ModalBody className="flex flex-col gap-3">
            {validateResult ? (
              <>
                <p className="text-base text-ink">{validateResult.experiment.name}</p>
                {validateResult.result.ok ? (
                  <Callout tone="success" title="校验通过">
                    依赖完整,可以发布给学生。
                  </Callout>
                ) : (
                  <Callout tone="warning" title="有需要处理的问题">
                    修正后再发布,否则学生进入实验时可能无法准备环境。
                  </Callout>
                )}
                <ExperimentValidationIssues issues={validateResult.result.issues} />
              </>
            ) : null}
          </ModalBody>
          <ModalFooter>
            {validateResult && !validateResult.result.ok ? (
              <Link to={`/teacher/experiments/${validateResult.experiment.id}`} className={buttonVariants({ variant: 'outline' })}>
                去编排里修改
              </Link>
            ) : (
              <Button variant="outline" onClick={() => setValidateResult(undefined)}>
                取消
              </Button>
            )}
            <Button
              variant="seal"
              disabled={!validateResult?.result.ok}
              onClick={() => validateResult && setPublishTarget(validateResult.experiment)}
            >
              确认发布
            </Button>
          </ModalFooter>
        </ModalContent>
      </Modal>

      <Modal
        open={publishTarget !== undefined}
        onOpenChange={(open) => !open && setPublishTarget(undefined)}
      >
        <ModalContent size="sm">
          <ModalHeader>
            <ModalTitle>发布「{publishTarget?.name}」?</ModalTitle>
            <ModalDescription>发布后学生可以进入实验并创建实验环境,实验配置将锁定。</ModalDescription>
          </ModalHeader>
          <ModalFooter>
            <Button variant="outline" onClick={() => setPublishTarget(undefined)}>
              取消
            </Button>
            <Button variant="seal" loading={working} onClick={() => void publishExperiment()}>
              发布实验
            </Button>
          </ModalFooter>
        </ModalContent>
      </Modal>
    </PageScaffold>
  )
}
