// 学业预警页(学生侧栏,/student/alerts)。
// GET /grade-center/warnings 的 student_id 缺省即本人(后端 ListWarnings 在缺省时填会话账号,
// 并校验学生只能读自己),前端不传该参数 —— 不把身份放进可传参位置。
// 预警明细 detail 是后端按类型写入的开放对象:只呈现已登记的键,未登记键不猜语义。

import { useCallback, useMemo, useState } from 'react'
import { useNavigate } from 'react-router'
import { CheckCheck, GraduationCap, ShieldCheck, TriangleAlert } from 'lucide-react'
import { GradeWarningStatus, type GradeWarning, type Semester } from '@chaimir/api-client'
import {
  Badge,
  Breadcrumb,
  Button,
  Callout,
  Card,
  CardBody,
  CardHeader,
  DescriptionList,
  PageBody,
  PageHeader,
  PageScaffold,
  PageSection,
  Pagination,
  Stat,
  StatusIndicator,
  Table,
  toast,
  type TableColumn,
} from '@chaimir/ui'
import { api } from '../../../../app/api'
import { ResourceState } from '../../../../components/ResourceState'
import { useAsyncResource, usePagedResource, useResourceTotal } from '../../../../hooks'
import { formatShortDateTime } from '../../../../utils/formatters'
import {
  gradeWarningDetailTerm,
  gradeWarningStatusLabel,
  gradeWarningTypeLabel,
} from '../../../../utils/labels/grade'
import { userFacingErrorMessage } from '../../../../utils/userFacingError'
import { gradeWarningStatusTone } from '../../statusPresentation'

/**
 * StudentAlertsPage 列出本人学业预警并提供确认。
 */
export default function StudentAlertsPage() {
  const navigate = useNavigate()
  const [ackingId, setAckingId] = useState<string>()
  const [actionError, setActionError] = useState<string>()

  const warnings = usePagedResource<GradeWarning>(
    (params) => api.grade.listWarnings(params),
    [],
  )
  const semesters = useAsyncResource(() => api.grade.listSemesters(), [], () => false)

  const semesterName = useMemo(
    () => new Map((semesters.data ?? []).map((semester: Semester) => [semester.id, semester.name])),
    [semesters.data],
  )

  /** ackWarning 确认预警:确认表示学生已知悉,不改变成绩本身。 */
  const ackWarning = useCallback(
    async (warning: GradeWarning) => {
      setAckingId(warning.id)
      setActionError(undefined)
      try {
        await api.grade.ackWarning(warning.id)
        toast.success('已确认这条预警')
        warnings.reload()
      } catch (error) {
        setActionError(userFacingErrorMessage(error, '确认没有成功,请稍后重试。'))
      } finally {
        setAckingId(undefined)
      }
    },
    [warnings],
  )

  // 指标带取服务端全量口径:预警会跨多页,用当前页数出来的「待确认」是错数
  const totalCount = useResourceTotal((params) => api.grade.listWarnings(params), [])
  const pendingCount = useResourceTotal(
    (params) => api.grade.listWarnings({ status: GradeWarningStatus.PENDING, ...params }),
    [],
  )
  const acknowledgedCount = useResourceTotal(
    (params) => api.grade.listWarnings({ status: GradeWarningStatus.ACKNOWLEDGED, ...params }),
    [],
  )

  const columns: TableColumn<GradeWarning>[] = [
    {
      key: 'type',
      header: '预警类型',
      render: (warning) => (
        <span className="font-medium text-ink">{gradeWarningTypeLabel(warning.type)}</span>
      ),
    },
    {
      key: 'semester_id',
      header: '学期',
      render: (warning) => (
        <span className="text-ink-sub">{semesterName.get(warning.semester_id) ?? '—'}</span>
      ),
    },
    {
      key: 'detail',
      header: '明细',
      render: (warning) => <WarningDetail detail={warning.detail} />,
    },
    {
      key: 'created_at',
      header: '预警时间',
      render: (warning) => (
        <span className="whitespace-nowrap font-mono text-xs tabular-nums text-ink-sub">
          {formatShortDateTime(warning.created_at)}
        </span>
      ),
    },
    {
      key: 'status',
      header: '状态',
      render: (warning) => (
        <StatusIndicator
          tone={gradeWarningStatusTone(warning.status)}
          label={gradeWarningStatusLabel(warning.status)}
        />
      ),
    },
    {
      key: 'actions',
      header: '操作',
      align: 'right',
      render: (warning) =>
        warning.status === GradeWarningStatus.PENDING ? (
          <Button
            variant="outline"
            size="sm"
            leftIcon={CheckCheck}
            loading={ackingId === warning.id}
            onClick={() => void ackWarning(warning)}
          >
            我已知悉
          </Button>
        ) : (
          <span className="text-sm text-ink-faint">已确认</span>
        ),
    },
  ]

  return (
    <PageScaffold>
      <PageHeader
        kicker={<Breadcrumb items={[{ label: '学业区' }, { label: '学业预警' }]} />}
        title="学业预警"
        description="学校根据你的成绩情况发出的提醒。确认后表示你已知悉,建议同时联系辅导员或任课老师。"
        icon={TriangleAlert}
      />

      <PageSection>
        <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-3">
          <Stat label="全部预警" value={totalCount ?? '—'} icon={TriangleAlert} />
          <Stat label="待确认" value={pendingCount ?? '—'} icon={ShieldCheck} hint="确认后不再提示" />
          <Stat label="已确认" value={acknowledgedCount ?? '—'} icon={CheckCheck} />
        </div>
      </PageSection>

      <PageBody rail={<AlertGuidanceCard onOpenGrades={() => navigate('/student/grades')} />}>
        <PageSection title="预警记录" description="按预警时间从新到旧排列。">
          <div className="flex flex-col gap-4">
            {actionError ? <Callout tone="danger">{actionError}</Callout> : null}

            <ResourceState
              resource={warnings}
              emptyIcon={ShieldCheck}
              emptyTitle="没有学业预警"
              emptyDescription="你的学业情况正常。保持下去,继续按课程节奏推进即可。"
              skeleton={<Table columns={columns} data={[]} rowKey={() => ''} loading />}
            >
              {(page) => (
                <>
                  <Table columns={columns} data={page.list} rowKey={(item) => item.id} />
                  <Pagination
                    page={warnings.page}
                    pageSize={warnings.pageSize}
                    total={warnings.total}
                    onPageChange={warnings.setPage}
                  />
                </>
              )}
            </ResourceState>
          </div>
        </PageSection>
      </PageBody>
    </PageScaffold>
  )
}

/**
 * WarningDetail 呈现预警明细。
 * detail 是开放对象:只显示已登记键(gradeWarningDetailTerm),
 * 未登记键不显示 —— 把内部键名抛到界面上等于让学生读数据库字段。
 */
function WarningDetail({ detail }: { detail: Record<string, unknown> }) {
  const entries = Object.entries(detail).flatMap(([key, value]) => {
    const term = gradeWarningDetailTerm(key)
    if (!term) return []
    if (typeof value !== 'string' && typeof value !== 'number') return []
    return [{ term, value: String(value) }]
  })

  if (entries.length === 0) return <span className="text-ink-sub">—</span>

  return (
    <div className="flex flex-wrap gap-1.5">
      {entries.map((entry) => (
        <Badge key={entry.term} tone="neutral">
          {entry.term} {entry.value}
        </Badge>
      ))}
    </div>
  )
}

/**
 * AlertGuidanceCard 给出收到预警后的具体下一步。
 * 空态与列表都需要引导:预警页最重要的不是列出问题,而是告诉学生该做什么。
 */
function AlertGuidanceCard({ onOpenGrades }: { onOpenGrades: () => void }) {
  const items = useMemo(
    () => [
      { term: '第一步', description: '在成绩中心确认是哪几门课程分数偏低' },
      { term: '第二步', description: '联系任课老师了解补救方式(补考、重修或补交作业)' },
      { term: '第三步', description: '如对成绩有疑问,在成绩中心提交申诉' },
    ],
    [],
  )

  return (
    <div className="flex flex-col gap-4">
      <Card>
        <CardHeader title="收到预警怎么办" description="按这三步处理,不要只是点确认。" />
        <CardBody className="flex flex-col gap-4">
          <DescriptionList dense items={items} />
          <Button variant="primary" leftIcon={GraduationCap} onClick={onOpenGrades}>
            打开成绩中心
          </Button>
        </CardBody>
      </Card>

      <Card>
        <CardHeader title="关于确认" />
        <CardBody>
          <Callout tone="info">
            点「我已知悉」只表示你看到了这条提醒,不会改变成绩,也不代表问题已经解决。
          </Callout>
        </CardBody>
      </Card>
    </div>
  )
}
