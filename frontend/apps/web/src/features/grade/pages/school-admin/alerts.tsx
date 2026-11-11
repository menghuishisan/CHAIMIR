// 学业预警页(校管侧栏,/school-admin/alerts)。设计 §7.5:
// 页头 +「执行扫描」,次要「调整预警规则」;工具条(状态、学生搜索,写进地址);表格(学生、学期、预警类型与说明、
// 状态、扫出时间)。扫描弹窗选范围后开始,完成后就地刷新并提示新增几条。学生姓名按页取完全部学生账号后解析。
//
// 预警由规则扫描产生:不及格课程数超阈值、平均学分绩点低于下限。
// 校管能看全校预警并主动触发扫描;确认动作属于学生本人(后端 ackWarning 在 student 组),
// 故本页不出现「确认」按钮 —— 那是学生在自己的学业预警页做的事。
//
// 预警规则的维护在成绩配置页(对齐清单 §3.3:预警规则是配置页内 Tab)。

import { useCallback, useMemo, useState } from 'react'
import { Link } from 'react-router'
import { RefreshCw, Settings2, TriangleAlert } from 'lucide-react'
import {
  GradeWarningStatus,
  BaseIdentity,
  type Account,
  type GradeWarning,
  type Semester,
} from '@chaimir/api-client'
import {
  Badge,
  Button,
  buttonVariants,
  Callout,
  DataPanel,
  FormField,
  Icon,
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
  Select,
  StatusIndicator,
  Table,
  toast,
  type TableColumn,
} from '@chaimir/ui'
import { api } from '../../../../app/api'
import { ResourceState } from '../../../../components/ResourceState'
import { useAsyncResource, usePagedResource, useResourceTotal, useUrlState } from '../../../../hooks'
import { AccountNameText } from '../../../identity/components/AccountNameText'
import { useAccountNames } from '../../../identity/useAccountNames'
import { formatDateTime, formatRelativeOccurrence } from '../../../../utils/formatters'
import {
  gradeWarningDetailTerm,
  gradeWarningStatusLabel,
  gradeWarningTypeLabel,
} from '../../../../utils/labels/grade'
import { userFacingErrorMessage } from '../../../../utils/userFacingError'
import { gradeWarningStatusTone } from '../../statusPresentation'

/** 地址参数:status 确认状态分段,student 选中的学生(空为全部);页码在 page */
const ALERTS_QUERY = {
  status: { defaultValue: 'all', allowed: ['all', 'pending', 'acknowledged'] },
  student: { defaultValue: '' },
} as const

const STATUS_SEGMENTS = [
  { value: 'all', label: '全部' },
  { value: 'pending', label: '待学生确认', status: GradeWarningStatus.PENDING },
  { value: 'acknowledged', label: '学生已确认', status: GradeWarningStatus.ACKNOWLEDGED },
] as const

/** 学生筛选里「全部学生」的取值(下拉不接受空串作选项值) */
const ALL_STUDENTS = 'all'

/**
 * SchoolAdminAlertsPage 呈现全校学业预警并承载扫描。
 */
export default function SchoolAdminAlertsPage() {
  const [query, setQuery] = useUrlState(ALERTS_QUERY)
  const [scanOpen, setScanOpen] = useState(false)
  const segment = STATUS_SEGMENTS.find((item) => item.value === query.status)

  const warnings = usePagedResource<GradeWarning>(
    (params) =>
      api.grade.listWarnings({
        student_id: query.student || undefined,
        status: segment && 'status' in segment ? segment.status : undefined,
        ...params,
      }),
    [query.status, query.student],
    { urlPageKey: 'page' },
  )

  // 分段上的数字取全量口径,不随学生筛选变化
  const totalCount = useResourceTotal((params) => api.grade.listWarnings(params), [], warnings.refreshVersion)
  const pendingCount = useResourceTotal(
    (params) => api.grade.listWarnings({ status: GradeWarningStatus.PENDING, ...params }),
    [],
    warnings.refreshVersion
  )
  const acknowledgedCount = useResourceTotal(
    (params) => api.grade.listWarnings({ status: GradeWarningStatus.ACKNOWLEDGED, ...params }),
    [],
    warnings.refreshVersion
  )
  const segmentCounts: Record<string, number | undefined> = { all: totalCount, pending: pendingCount, acknowledged: acknowledgedCount }

  // 预警只回 student_id / semester_id,姓名与学期名在此解析;学生超过一页时按页取完(设计 §10)
  const students = useAccountNames(BaseIdentity.STUDENT)

  const semesters = useAsyncResource(
    () => api.grade.listSemesters(),
    [],
    () => false
  )


  const semesterNameById = useMemo(
    () => new Map((semesters.data ?? []).map((semester: Semester) => [semester.id, semester.name])),
    [semesters.data]
  )

  const columns: TableColumn<GradeWarning>[] = [
    {
      key: 'student_id',
      header: '学生',
      render: (warning) => <AccountNameText name={students.lookup(warning.student_id)} />,
    },
    {
      key: 'semester_id',
      header: '学期',
      render: (warning) => (
        <span className="text-sm text-ink-sub">
          {semesterNameById.get(warning.semester_id) ?? '未登记学期'}
        </span>
      ),
    },
    {
      key: 'type',
      header: '预警类型与说明',
      render: (warning) => (
        <span className="flex flex-col gap-1">
          <span className="font-medium text-warning">{gradeWarningTypeLabel(warning.type)}</span>
          <WarningDetail detail={warning.detail} />
        </span>
      ),
    },
    {
      key: 'created_at',
      header: '扫出时间',
      render: (warning) => (
        <span title={formatDateTime(warning.created_at)} className="whitespace-nowrap text-ink-sub">
          {formatRelativeOccurrence(warning.created_at)}
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
  ]

  return (
    <PageScaffold>
      <PageHeader
        title="学业预警"
        actions={
          <div className="flex flex-wrap items-center gap-2">
            <Link to="/school-admin/grade-settings#warning-rules" className={buttonVariants({ variant: 'outline' })}>
              <Icon icon={Settings2} size="sm" />
              调整预警规则
            </Link>
            <Button variant="primary" leftIcon={RefreshCw} onClick={() => setScanOpen(true)}>
              执行扫描
            </Button>
          </div>
        }
      />

      <div className="mb-4 flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
        <SegmentedControl
          aria-label="按确认状态筛选"
          options={STATUS_SEGMENTS.map((item) => ({ value: item.value, label: item.label, count: segmentCounts[item.value] }))}
          value={query.status}
          onValueChange={(value) => setQuery({ status: value })}
          className="max-w-full overflow-x-auto"
        />
        <Select
          aria-label="按学生筛选"
          value={query.student || ALL_STUDENTS}
          onValueChange={(value) => setQuery({ student: value === ALL_STUDENTS ? '' : value })}
          options={[
            { value: ALL_STUDENTS, label: '全部学生' },
            ...students.list.map((account) => ({ value: account.id, label: account.no ? `${account.name} · ${account.no}` : account.name })),
          ]}
          className="w-full sm:w-64"
        />
      </div>

      <DataPanel
        label="预警记录"
        footer={
          <Pagination
            page={warnings.page}
            pageSize={warnings.pageSize}
            total={warnings.total}
            onPageChange={warnings.setPage}
          />
        }
      >
          <ResourceState
            resource={warnings}
            emptyIcon={TriangleAlert}
            emptyTitle={query.student || query.status !== 'all' ? '没有符合条件的预警' : '暂无学业预警'}
            emptyDescription={
              query.student || query.status !== 'all'
                ? '换个学生或状态看看,也可以清空条件查看全部。'
                : '执行扫描后,达到预警条件的学生会出现在这里。'
            }
            skeleton={<Table columns={columns} data={[]} rowKey={() => ''} loading elevated={false} />}
          >
            {(page) => (
              <Table
                columns={columns}
                data={page.list}
                rowKey={(item) => item.id}
                elevated={false}
                // <md 换行卡(§6.4.1 规则 3):学生名一行、预警原因与时间一行
                mobileCard={(item) => ({
                  title: (() => {
                    const name = students.lookup(item.student_id)
                    return name.state === 'found' ? name.account.name : name.state === 'missing' ? '已移除的账号' : '…'
                  })(),
                  meta: `${gradeWarningTypeLabel(item.type)} · ${formatRelativeOccurrence(item.created_at)}`,
                  badge: (
                    <StatusIndicator
                      tone={gradeWarningStatusTone(item.status)}
                      label={gradeWarningStatusLabel(item.status)}
                    />
                  ),
                })}
              />
            )}
          </ResourceState>
      </DataPanel>

      {scanOpen ? (
        <ScanWarningsModal
          students={students.list}
          semesters={semesters.data ?? []}
          onClose={() => setScanOpen(false)}
          onDone={() => {
            setScanOpen(false)
            warnings.reload()
          }}
        />
      ) : null}
    </PageScaffold>
  )
}

/**
 * WarningDetail 呈现预警明细。
 * detail 是后端按预警类型写入的开放对象:只呈现已登记的键,
 * 未登记键不猜测语义、不把内部键名抛到界面上。
 */
function WarningDetail({ detail }: { detail: Record<string, unknown> }) {
  const items = useMemo(
    () =>
      Object.entries(detail)
        .map(([key, value]) => ({ term: gradeWarningDetailTerm(key), value }))
        .filter((item): item is { term: string; value: unknown } => item.term !== undefined),
    [detail]
  )

  if (items.length === 0) return <span className="text-ink-sub">—</span>

  return (
    <div className="flex flex-wrap gap-1.5">
      {items.map((item) => (
        <Badge key={item.term} tone="neutral">
          {item.term} {formatDetailValue(item.value)}
        </Badge>
      ))}
    </div>
  )
}

/** formatDetailValue 把明细值转成可读文本;复杂类型给占位,不抛 JSON 到界面。 */
function formatDetailValue(value: unknown): string {
  if (typeof value === 'number') return Number.isInteger(value) ? String(value) : value.toFixed(2)
  if (typeof value === 'string') return value
  return '—'
}

interface ScanWarningsModalProps {
  students: Account[]
  semesters: Semester[]
  onClose: () => void
  onDone: () => void
}

/**
 * ScanWarningsModal 触发预警扫描。
 * 学生与学期都可缺省:缺省时按当前规则扫描全校当前学期。
 */
function ScanWarningsModal({ students, semesters, onClose, onDone }: ScanWarningsModalProps) {
  const [studentId, setStudentId] = useState('')
  const [semesterId, setSemesterId] = useState('')
  const [formError, setFormError] = useState<string>()
  const [working, setWorking] = useState(false)

  const submit = useCallback(async () => {
    setFormError(undefined)
    setWorking(true)
    try {
      const result = await api.grade.scanWarnings({
        student_id: studentId || undefined,
        semester_id: semesterId || undefined,
      })
      toast.success(
        result.created > 0
          ? `扫描了 ${result.scanned} 名学生,新增 ${result.created} 条预警`
          : `扫描了 ${result.scanned} 名学生,没有新增预警`
      )
      onDone()
    } catch (error) {
      setFormError(userFacingErrorMessage(error, '扫描没有完成,请稍后重试。'))
    } finally {
      setWorking(false)
    }
  }, [onDone, semesterId, studentId])

  return (
    <Modal open onOpenChange={(open) => !open && onClose()}>
      <ModalContent size="lg">
        <ModalHeader>
          <ModalTitle>执行预警扫描</ModalTitle>
          <ModalDescription>
            按当前预警规则重新评估学生的学业状况。达到条件的会产生预警并通知学生。
          </ModalDescription>
        </ModalHeader>
        <ModalBody className="flex flex-col gap-4">
          <FormField label="学生范围" htmlFor="scan-student" helper="不选则扫描全校学生">
            <Select
              id="scan-student"
              options={[
                { value: '', label: '全校学生' },
                ...students.map((account) => ({
                  value: account.id,
                  label: account.no ? `${account.name} · ${account.no}` : account.name,
                })),
              ]}
              value={studentId}
              placeholder="全校学生"
              onValueChange={setStudentId}
            />
          </FormField>

          <FormField label="学期" htmlFor="scan-semester" helper="不选则按当前学期">
            <Select
              id="scan-semester"
              options={[
                { value: '', label: '当前学期' },
                ...semesters.map((semester) => ({ value: semester.id, label: semester.name })),
              ]}
              value={semesterId}
              placeholder="当前学期"
              onValueChange={setSemesterId}
            />
          </FormField>

          <Callout tone="info">
            扫描会给新产生预警的学生发通知。重复扫描不会重复产生同一条预警。
          </Callout>

          {formError ? <Callout tone="danger">{formError}</Callout> : null}
        </ModalBody>
        <ModalFooter>
          <Button variant="outline" onClick={onClose}>
            取消
          </Button>
          <Button
            variant="primary"
            leftIcon={RefreshCw}
            loading={working}
            onClick={() => void submit()}
          >
            开始扫描
          </Button>
        </ModalFooter>
      </ModalContent>
    </Modal>
  )
}
