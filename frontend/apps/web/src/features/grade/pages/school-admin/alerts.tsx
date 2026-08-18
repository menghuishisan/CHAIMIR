// 学业预警页(校管侧栏,/school-admin/alerts)。
//
// 预警由规则扫描产生:不及格课程数超阈值、平均学分绩点低于下限。
// 校管能看全校预警并主动触发扫描;确认动作属于学生本人(后端 ackWarning 在 student 组),
// 故本页不出现「确认」按钮 —— 那是学生在自己的学业预警页做的事。
//
// 预警规则的维护在成绩配置页(对齐清单 §3.3:预警规则是配置页内 Tab)。

import { useCallback, useMemo, useState } from 'react'
import { useNavigate } from 'react-router'
import { RefreshCw, Settings2, TriangleAlert, UserCheck } from 'lucide-react'
import {
  GradeWarningStatus,
  BaseIdentity,
  PAGINATION_MAX_SIZE,
  type Account,
  type GradeWarning,
  type Semester,
} from '@chaimir/api-client'
import {
  Badge,
  Breadcrumb,
  Button,
  Callout,
  FilterBar,
  FilterField,
  FormField,
  Modal,
  ModalBody,
  ModalContent,
  ModalDescription,
  ModalFooter,
  ModalHeader,
  ModalTitle,
  PageHeader,
  PageScaffold,
  PageSection,
  Pagination,
  SegmentedControl,
  Select,
  Stat,
  StatusIndicator,
  Table,
  toast,
  type TableColumn,
} from '@chaimir/ui'
import { api } from '../../../../app/api'
import { ResourceState } from '../../../../components/ResourceState'
import { useAsyncResource, usePagedResource, useResourceTotal } from '../../../../hooks'
import { formatDateTime } from '../../../../utils/formatters'
import {
  gradeWarningDetailTerm,
  gradeWarningStatusLabel,
  gradeWarningTypeLabel,
} from '../../../../utils/labels/grade'
import { userFacingErrorMessage } from '../../../../utils/userFacingError'
import { gradeWarningStatusTone } from '../../statusPresentation'

/** 状态筛选项:值为空串表示不过滤;学生与状态都由服务端过滤。 */
const STATUS_FILTERS = [
  { value: '', label: '全部' },
  { value: String(GradeWarningStatus.PENDING), label: '待学生确认' },
  { value: String(GradeWarningStatus.ACKNOWLEDGED), label: '学生已确认' },
  { value: String(GradeWarningStatus.NOTIFY_FAILED), label: '提醒未送达' },
] as const

/**
 * SchoolAdminAlertsPage 呈现全校学业预警并承载扫描。
 */
export default function SchoolAdminAlertsPage() {
  const navigate = useNavigate()
  const [studentFilter, setStudentFilter] = useState<string>('')
  const [statusFilter, setStatusFilter] = useState<string>('')
  const [scanOpen, setScanOpen] = useState(false)

  const warnings = usePagedResource<GradeWarning>(
    (params) =>
      api.grade.listWarnings({
        student_id: studentFilter || undefined,
        status: statusFilter ? (Number(statusFilter) as GradeWarningStatus) : undefined,
        ...params,
      }),
    [studentFilter, statusFilter]
  )

  // 指标带取全量口径,不随下方筛选变化:它回答「全校预警总体分布」
  const totalCount = useResourceTotal((params) => api.grade.listWarnings(params), [])
  const pendingCount = useResourceTotal(
    (params) => api.grade.listWarnings({ status: GradeWarningStatus.PENDING, ...params }),
    []
  )
  const acknowledgedCount = useResourceTotal(
    (params) => api.grade.listWarnings({ status: GradeWarningStatus.ACKNOWLEDGED, ...params }),
    []
  )

  // 预警只回 student_id / semester_id,姓名与学期名在此解析
  const students = useAsyncResource(
    () =>
      api.identity.getAccounts({
        role: BaseIdentity.STUDENT,
        page: 1,
        size: PAGINATION_MAX_SIZE,
      }),
    [],
    () => false
  )

  const semesters = useAsyncResource(
    () => api.grade.listSemesters(),
    [],
    () => false
  )

  const studentById = useMemo(
    () => new Map((students.data?.list ?? []).map((account: Account) => [account.id, account])),
    [students.data]
  )

  const semesterNameById = useMemo(
    () => new Map((semesters.data ?? []).map((semester: Semester) => [semester.id, semester.name])),
    [semesters.data]
  )

  const columns: TableColumn<GradeWarning>[] = [
    {
      key: 'student_id',
      header: '学生',
      render: (warning) => {
        const account = studentById.get(warning.student_id)
        return (
          <div className="min-w-0">
            <div className="truncate font-medium text-ink">
              {account ? account.name : '已离校学生'}
            </div>
            {account?.no ? (
              <div className="truncate font-mono text-xs text-ink-sub">{account.no}</div>
            ) : null}
          </div>
        )
      },
    },
    {
      key: 'type',
      header: '预警类型',
      render: (warning) => <Badge tone="warning">{gradeWarningTypeLabel(warning.type)}</Badge>,
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
      key: 'detail',
      header: '预警明细',
      render: (warning) => <WarningDetail detail={warning.detail} />,
    },
    {
      key: 'created_at',
      header: '产生时间',
      render: (warning) => (
        <span className="whitespace-nowrap font-mono text-xs tabular-nums text-ink-sub">
          {formatDateTime(warning.created_at)}
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
        kicker={<Breadcrumb items={[{ label: '教务与成绩' }, { label: '学业预警' }]} />}
        title="学业预警"
        description="按预警规则扫描出的学业风险。学生在自己的页面确认后状态变为已确认。"
        icon={TriangleAlert}
        actions={
          <div className="flex flex-wrap items-center gap-2">
            <Button
              variant="outline"
              leftIcon={Settings2}
              onClick={() => navigate('/school-admin/grade-settings')}
            >
              调整预警规则
            </Button>
            <Button variant="primary" leftIcon={RefreshCw} onClick={() => setScanOpen(true)}>
              执行扫描
            </Button>
          </div>
        }
      />

      <PageSection>
        <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-3">
          <Stat
            label="预警总数"
            value={totalCount ?? '—'}
            icon={TriangleAlert}
            hint="不受下方筛选影响"
          />
          <Stat
            label="待确认"
            value={pendingCount ?? '—'}
            icon={TriangleAlert}
            hint="学生尚未查看"
          />
          <Stat label="已确认" value={acknowledgedCount ?? '—'} icon={UserCheck} />
        </div>
      </PageSection>

      <PageSection
        title="预警记录"
        description={`共 ${warnings.total} 条。确认动作由学生本人完成。`}
      >
        <div className="flex flex-col gap-4">
          <FilterBar label="预警记录筛选">
            <FilterField label="确认状态" group>
              <SegmentedControl
                aria-label="按确认状态筛选"
                size="sm"
                options={STATUS_FILTERS.map((item) => ({ value: item.value, label: item.label }))}
                value={statusFilter}
                onValueChange={setStatusFilter}
              />
            </FilterField>
            <FilterField label="学生" htmlFor="alerts-student">
              <Select
                id="alerts-student"
                options={[
                  { value: '', label: '全部学生' },
                  ...(students.data?.list ?? []).map((account: Account) => ({
                    value: account.id,
                    label: account.no ? `${account.name} · ${account.no}` : account.name,
                  })),
                ]}
                value={studentFilter}
                placeholder="全部学生"
                onValueChange={setStudentFilter}
              />
            </FilterField>
          </FilterBar>

          <ResourceState
            resource={warnings}
            emptyIcon={TriangleAlert}
            emptyTitle={studentFilter || statusFilter ? '没有符合条件的预警' : '暂无学业预警'}
            emptyDescription={
              studentFilter || statusFilter
                ? '换个学生或状态看看,也可以清空条件查看全部。'
                : '执行扫描后,达到预警条件的学生会出现在这里。'
            }
            skeleton={<Table columns={columns} data={[]} rowKey={() => ''} loading />}
          >
            {(page) => (
              <div className="flex flex-col gap-4">
                <Table columns={columns} data={page.list} rowKey={(item) => item.id} />
                <Pagination
                  page={warnings.page}
                  pageSize={warnings.pageSize}
                  total={warnings.total}
                  onPageChange={warnings.setPage}
                />
              </div>
            )}
          </ResourceState>
        </div>
      </PageSection>

      {scanOpen ? (
        <ScanWarningsModal
          students={students.data?.list ?? []}
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
