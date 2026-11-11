// 漏洞题工坊页(教师深页,/teacher/vuln-workshop)。设计 §7.4:
// 页头 +「手工录入漏洞题」;页头下一条四步流程说明(还没有任何漏洞题时展开,之后收成一行);
// 左栏「漏洞来源」列表(启用状态、上次同步时间、「同步案例」、「修改配置」,「添加来源」);
// 右栏漏洞题表格(名称、状态、验证结果),状态与验证结果各一个筛选(都写进地址);
// 行内按状态给一个下一步:未验证或未通过→「预验证」,验证通过的草稿→「固化到题库」。
// 预验证在右侧大抽屉中进行,显示过程和结果;整行也打开它(已固化的只读查看验证记录)。
//
// 边界:同步、预验证、固化都是租户内的出题工作流(后端 teacher 组),
// 平台端只做全局漏洞源的查看与新增,不出现这三个动作。

import { useCallback, useMemo, useState } from 'react'
import { Bug, Database, FlaskConical, Plus, RefreshCw, Settings2, ShieldCheck } from 'lucide-react'
import {
  VulnProblemStatus,
  VulnPrevalidateStatus,
  type VulnProblem,
  type VulnSource,
} from '@chaimir/api-client'
import {
  Badge,
  Button,
  Callout,
  DataPanel,
  FilterBar,
  FilterField,
  PageBody,
  PageHeader,
  PageScaffold,
  Pagination,
  SegmentedControl,
  Select,
  Skeleton,
  StatusIndicator,
  Table,
  toast,
  type TableColumn,
} from '@chaimir/ui'
import { api } from '../../../../app/api'
import { ResourceError, ResourceState } from '../../../../components/ResourceState'
import { useAsyncResource, usePagedResource, usePageLocation, useResourceTotal, useUrlState } from '../../../../hooks'
import { formatDateTime, formatRelativeOccurrence } from '../../../../utils/formatters'
import {
  vulnLevelLabel,
  vulnPrevalidateStatusLabel,
  vulnProblemStatusLabel,
  vulnSourceTypeLabel,
} from '../../../../utils/labels/contest'
import { userFacingErrorMessage } from '../../../../utils/userFacingError'
import { VulnSourceFormModal } from '../../components/VulnSourceFormModal'
import { vulnPrevalidateStatusTone, vulnProblemStatusTone } from '../../statusPresentation'
import { VULN_SOURCE_CONFIG_FIELDS } from '../../vulnerabilityContract'
import { readString } from '../../jsonReaders'
import { VulnProblemFormModal } from './vuln-problem-form'
import { isVulnPrevalidateDetailConsistent, VulnPrevalidateModal } from './vuln-prevalidate'

/** 地址参数:status 草稿状态,check 验证结果,source 来源(空为全部);页码在 page */
const WORKSHOP_QUERY = {
  status: { defaultValue: 'all', allowed: ['all', 'draft', 'finalized', 'discarded'] },
  check: { defaultValue: 'all', allowed: ['all', 'pending', 'passed', 'failed'] },
  source: { defaultValue: '' },
} as const

const STATUS_SEGMENTS = [
  { value: 'all', label: '全部' },
  { value: 'draft', label: '草稿', status: VulnProblemStatus.DRAFT },
  { value: 'finalized', label: '已固化', status: VulnProblemStatus.FINALIZED },
  { value: 'discarded', label: '已弃用', status: VulnProblemStatus.DISCARDED },
] as const

const CHECK_SEGMENTS = [
  { value: 'all', label: '全部' },
  { value: 'pending', label: '尚未验证', status: VulnPrevalidateStatus.PENDING },
  { value: 'passed', label: '验证通过', status: VulnPrevalidateStatus.PASSED },
  { value: 'failed', label: '验证未通过', status: VulnPrevalidateStatus.FAILED },
] as const

/** 来源筛选里「全部来源」的取值(下拉不接受空串作选项值) */
const ALL_SOURCES = 'all'

/** displayCheckStatus 验证记录与状态不一致时按未通过显示(不能拿一条对不上的记录去固化) */
function displayCheckStatus(problem: VulnProblem): VulnPrevalidateStatus {
  return problem.prevalidate_status === VulnPrevalidateStatus.PASSED && !isVulnPrevalidateDetailConsistent(problem)
    ? VulnPrevalidateStatus.FAILED
    : problem.prevalidate_status
}

/** canFinalize 验证通过(且记录一致)的草稿才能固化 */
function canFinalize(problem: VulnProblem): boolean {
  return (
    problem.status === VulnProblemStatus.DRAFT &&
    problem.prevalidate_status === VulnPrevalidateStatus.PASSED &&
    isVulnPrevalidateDetailConsistent(problem)
  )
}

/**
 * TeacherVulnWorkshopPage 承载漏洞源维护与漏洞题转化。
 */
export default function TeacherVulnWorkshopPage() {
  const [query, setQuery] = useUrlState(WORKSHOP_QUERY)
  const [sourceForm, setSourceForm] = useState<{ source?: VulnSource } | undefined>()
  const [problemFormOpen, setProblemFormOpen] = useState(false)
  const [prevalidateTarget, setPrevalidateTarget] = useState<VulnProblem>()
  const [finalizingId, setFinalizingId] = useState<string>()
  const [actionError, setActionError] = useState<string>()

  const sources = useAsyncResource(() => api.contest.listVulnSources(), [], () => false)
  const sourceList = useMemo(() => sources.data ?? [], [sources.data])
  const sourceNameById = useMemo(() => new Map(sourceList.map((source) => [source.id, source.name])), [sourceList])

  const statusFilter = STATUS_SEGMENTS.find((item) => item.value === query.status)
  const checkFilter = CHECK_SEGMENTS.find((item) => item.value === query.check)
  const problems = usePagedResource<VulnProblem>(
    (params) =>
      api.contest.listVulnProblems({
        source_id: query.source || undefined,
        status: statusFilter && 'status' in statusFilter ? statusFilter.status : undefined,
        prevalidate_status: checkFilter && 'status' in checkFilter ? checkFilter.status : undefined,
        ...params,
      }),
    [query.check, query.source, query.status],
    { urlPageKey: 'page' },
  )
  // 流程说明是否展开看全量:一条漏洞题都没有时展开
  const totalCount = useResourceTotal((params) => api.contest.listVulnProblems(params), [], problems.refreshVersion)

  /** finalize 把预验证通过的草稿固化到题库,固化后即可作为赛题引用 */
  const finalize = useCallback(
    async (problem: VulnProblem) => {
      setFinalizingId(problem.id)
      setActionError(undefined)
      try {
        const finalized = await api.contest.finalizeVulnProblem(problem.id)
        toast.success(
          finalized.content_item_code ? `已固化到题库,题目编号 ${finalized.content_item_code}` : '已固化到题库',
        )
        problems.reload()
      } catch (error) {
        setActionError(userFacingErrorMessage(error, '固化没有成功,请稍后重试。'))
      } finally {
        setFinalizingId(undefined)
      }
    },
    [problems],
  )

  /** NextAction 一条漏洞题当前的下一步:可固化→「固化到题库」,其余草稿→「预验证」,已固化或弃用没有 */
  const nextAction = (problem: VulnProblem) =>
    canFinalize(problem) ? (
      <Button
        variant="secondary"
        size="sm"
        leftIcon={ShieldCheck}
        loading={finalizingId === problem.id}
        onClick={() => void finalize(problem)}
      >
        固化到题库
      </Button>
    ) : problem.status === VulnProblemStatus.DRAFT ? (
      <Button variant="secondary" size="sm" leftIcon={FlaskConical} onClick={() => setPrevalidateTarget(problem)}>
        预验证
      </Button>
    ) : null

  const columns: TableColumn<VulnProblem>[] = [
    {
      key: 'title',
      header: '漏洞题',
      primary: true,
      render: (problem) => (
        <span className="block min-w-0">
          <span className="block truncate font-medium text-ink">{problem.title}</span>
          <span className="block truncate text-xs text-ink-sub">
            {problem.source_id ? (sourceNameById.get(problem.source_id) ?? '已删除的来源') : '手工录入'} ·{' '}
            {vulnLevelLabel(problem.level)}
          </span>
        </span>
      ),
    },
    {
      key: 'status',
      header: '状态',
      render: (problem) => (
        <span className="flex flex-col gap-0.5">
          <StatusIndicator tone={vulnProblemStatusTone(problem.status)} label={vulnProblemStatusLabel(problem.status)} />
          {problem.content_item_code ? (
            <span className="font-mono text-xs text-ink-sub">题库 {problem.content_item_code}</span>
          ) : null}
        </span>
      ),
    },
    {
      key: 'check',
      header: '验证结果',
      render: (problem) => (
        <StatusIndicator
          tone={vulnPrevalidateStatusTone(displayCheckStatus(problem))}
          label={vulnPrevalidateStatusLabel(displayCheckStatus(problem))}
        />
      ),
    },
    {
      key: 'next',
      header: '下一步',
      align: 'right',
      render: nextAction,
    },
  ]

  usePageLocation([{ label: '赛事组织', to: '/teacher/contests' }, { label: '漏洞题工坊' }])

  const filtered = query.status !== 'all' || query.check !== 'all' || query.source !== ''

  return (
    <PageScaffold>
      <PageHeader
        title="漏洞题工坊"
        actions={
          <Button variant="primary" leftIcon={Plus} onClick={() => setProblemFormOpen(true)}>
            手工录入漏洞题
          </Button>
        }
      />

      <WorkflowGuide expanded={totalCount === 0} />

      {actionError ? (
        <Callout tone="danger" className="mb-4">
          {actionError}
        </Callout>
      ) : null}

      {/* 左栏来源、右栏漏洞题:漏洞题是主任务,先出现在读屏顺序里;宽屏用 order 把来源放到左侧 */}
      <PageBody
        className="lg:flex-row-reverse"
        rail={
          <SourceList
            sources={sources}
            onAdd={() => setSourceForm({})}
            onEdit={(source) => setSourceForm({ source })}
            onSynced={() => {
              sources.reload()
              problems.reload()
            }}
          />
        }
      >
        <DataPanel
          label="漏洞题"
          filter={
            <FilterBar label="漏洞题筛选">
              <FilterField label="状态" group>
                <SegmentedControl
                  aria-label="按状态筛选"
                  size="sm"
                  options={STATUS_SEGMENTS.map((item) => ({ value: item.value, label: item.label }))}
                  value={query.status}
                  onValueChange={(value) => setQuery({ status: value })}
                />
              </FilterField>
              <FilterField label="验证结果" group>
                <SegmentedControl
                  aria-label="按验证结果筛选"
                  size="sm"
                  options={CHECK_SEGMENTS.map((item) => ({ value: item.value, label: item.label }))}
                  value={query.check}
                  onValueChange={(value) => setQuery({ check: value })}
                />
              </FilterField>
              {sourceList.length > 0 ? (
                <FilterField label="来源" htmlFor="vuln-source-filter">
                  <Select
                    id="vuln-source-filter"
                    options={[
                      { value: ALL_SOURCES, label: '全部来源' },
                      ...sourceList.map((source) => ({ value: source.id, label: source.name })),
                    ]}
                    value={query.source || ALL_SOURCES}
                    onValueChange={(value) => setQuery({ source: value === ALL_SOURCES ? '' : value })}
                    className="w-full sm:w-48"
                  />
                </FilterField>
              ) : null}
            </FilterBar>
          }
          footer={
            <Pagination page={problems.page} pageSize={problems.pageSize} total={problems.total} onPageChange={problems.setPage} />
          }
        >
          <ResourceState
            resource={problems}
            emptyIcon={Bug}
            emptyTitle={filtered ? '这个条件下没有漏洞题' : '还没有漏洞题'}
            emptyDescription={
              filtered ? '换个条件再看,或清空筛选查看全部。' : '从左侧的来源同步案例,或手工录入一条漏洞题。'
            }
            emptyAction={
              filtered ? (
                <Button variant="outline" onClick={() => setQuery({ status: 'all', check: 'all', source: '' })}>
                  清空筛选
                </Button>
              ) : undefined
            }
            skeleton={<Table columns={columns} data={[]} rowKey={() => ''} loading elevated={false} />}
          >
            {(page) => (
              <Table
                columns={columns}
                data={page.list}
                rowKey={(item) => item.id}
                elevated={false}
                onRowClick={setPrevalidateTarget}
                // <md 换行卡(§6.4.1 规则 3):题名一行、状态与验证结果一行,下一步在右
                mobileCard={(item) => ({
                  title: item.title,
                  meta: `${vulnProblemStatusLabel(item.status)} · ${vulnPrevalidateStatusLabel(displayCheckStatus(item))}`,
                  action: nextAction(item) ?? undefined,
                })}
              />
            )}
          </ResourceState>
        </DataPanel>
      </PageBody>

      {sourceForm ? (
        <VulnSourceFormModal
          source={sourceForm.source}
          onClose={() => setSourceForm(undefined)}
          onSaved={() => {
            setSourceForm(undefined)
            sources.reload()
          }}
        />
      ) : null}

      {problemFormOpen ? (
        <VulnProblemFormModal
          sources={sourceList}
          onClose={() => setProblemFormOpen(false)}
          onSaved={() => {
            setProblemFormOpen(false)
            problems.reload()
          }}
        />
      ) : null}

      {prevalidateTarget ? (
        <VulnPrevalidateModal
          problem={prevalidateTarget}
          onClose={() => setPrevalidateTarget(undefined)}
          onDone={() => problems.reload()}
        />
      ) : null}
    </PageScaffold>
  )
}

/** 四步流程:配置来源 → 同步案例 → 预验证 → 固化到题库 */
const WORKFLOW_STEPS = [
  { title: '配置来源', text: '添加返回漏洞案例的公开接口,也可以跳过这一步直接手工录入。' },
  { title: '同步案例', text: '从来源拉取案例,每条生成一份漏洞题草稿。' },
  { title: '预验证', text: '在隔离环境里跑正反两遍:攻击要打通,不攻击时不能误判。' },
  { title: '固化到题库', text: '验证通过后写入本校题库并发布,答案与判题配置对学生不可见。' },
] as const

/** WorkflowGuide 四步流程说明:还没有任何漏洞题时展开成四格,之后收成一行(总数读取中也按一行) */
function WorkflowGuide({ expanded }: { expanded: boolean }) {
  if (!expanded) {
    return (
      <p className="mb-5 text-sm text-ink-sub">
        {WORKFLOW_STEPS.map((step, index) => `${index + 1} ${step.title}`).join(' → ')}
        。固化后的题目可以在赛事里作为赛题引用。
      </p>
    )
  }
  return (
    <ol aria-label="漏洞题制作流程" className="mb-6 grid gap-3 sm:grid-cols-2 xl:grid-cols-4">
      {WORKFLOW_STEPS.map((step, index) => (
        <li key={step.title} className="rounded-lg bg-surface p-4 shadow-xs">
          <p className="flex items-center gap-2 text-sm font-semibold text-ink">
            <span className="flex size-6 shrink-0 items-center justify-center rounded-full bg-primary-soft text-xs text-primary">
              {index + 1}
            </span>
            {step.title}
          </p>
          <p className="mt-2 text-sm text-ink-sub">{step.text}</p>
        </li>
      ))}
    </ol>
  )
}

interface SourceListProps {
  sources: ReturnType<typeof useAsyncResource<VulnSource[]>>
  onAdd: () => void
  onEdit: (source: VulnSource) => void
  onSynced: () => void
}

/** SourceList 左栏「漏洞来源」:标题 +「添加来源」,每个来源一行 */
function SourceList({ sources, onAdd, onEdit, onSynced }: SourceListProps) {
  return (
    <section aria-labelledby="vuln-sources-title" className="rounded-lg bg-surface p-4 shadow-xs">
      <div className="mb-2 flex items-center justify-between gap-2">
        <h2 id="vuln-sources-title" className="text-md font-semibold text-ink">
          漏洞来源
        </h2>
        <Button variant="ghost" size="sm" leftIcon={Plus} onClick={onAdd}>
          添加来源
        </Button>
      </div>
      {sources.status === 'loading' ? (
        <div aria-busy="true">
          <Skeleton variant="line" lines={4} />
        </div>
      ) : sources.status === 'error' ? (
        <ResourceError
          message={sources.error?.message ?? '来源暂时读取不到'}
          traceId={sources.error?.traceId}
          onRetry={sources.reload}
        />
      ) : (sources.data ?? []).length === 0 ? (
        <div className="flex flex-col items-start gap-2 py-2">
          <p className="flex items-center gap-2 text-sm text-ink-sub">
            <Database aria-hidden className="size-4 text-ink-faint" />
            还没有配置来源。
          </p>
          <p className="text-sm text-ink-sub">配置后可以批量同步漏洞案例;也可以先手工录入单个漏洞题。</p>
        </div>
      ) : (
        <ul className="divide-y divide-line">
          {(sources.data ?? []).map((source) => (
            <SourceRow key={source.id} source={source} onEdit={() => onEdit(source)} onSynced={onSynced} />
          ))}
        </ul>
      )}
    </section>
  )
}

interface SourceRowProps {
  source: VulnSource
  onEdit: () => void
  onSynced: () => void
}

/**
 * SourceRow 一个漏洞来源:名称与启用状态、类型与上次同步时间、「同步案例」「修改配置」。
 * 同步是拉取外部数据的动作,可能耗时,按钮带加载态,完成后提示新增条数;失败就近显示在这一行。
 * 停用或没配来源地址时不能同步,原因写在按钮旁。
 */
function SourceRow({ source, onEdit, onSynced }: SourceRowProps) {
  const [syncing, setSyncing] = useState(false)
  const [syncError, setSyncError] = useState<string>()
  const endpoint = readString(source.config, VULN_SOURCE_CONFIG_FIELDS.endpoint)
  const blocked = !source.enabled ? '来源已停用,启用后才能同步' : endpoint === '' ? '补充来源地址后才能同步' : undefined

  const sync = useCallback(async () => {
    setSyncing(true)
    setSyncError(undefined)
    try {
      const created = await api.contest.syncVulnSource(source.id)
      toast.success(created.length > 0 ? `已同步 ${created.length} 条漏洞案例` : '同步完成,来源里没有新案例')
      onSynced()
    } catch (error) {
      setSyncError(userFacingErrorMessage(error, '同步没有成功,请检查来源配置后重试。'))
    } finally {
      setSyncing(false)
    }
  }, [onSynced, source.id])

  return (
    <li className="flex flex-col gap-2 py-3">
      <div className="flex items-start justify-between gap-2">
        <span className="min-w-0">
          <span className="block truncate text-sm font-medium text-ink">{source.name}</span>
          <span className="block text-xs text-ink-sub">{vulnSourceTypeLabel(source.type)}</span>
        </span>
        <Badge tone={source.enabled ? 'success' : 'neutral'}>{source.enabled ? '已启用' : '已停用'}</Badge>
      </div>
      <p className="text-xs text-ink-sub" title={source.last_sync_at ? formatDateTime(source.last_sync_at) : undefined}>
        {source.last_sync_at ? `上次同步 ${formatRelativeOccurrence(source.last_sync_at)}` : '尚未同步'}
      </p>
      <div className="flex flex-wrap items-center gap-2">
        <Button
          variant="outline"
          size="sm"
          leftIcon={RefreshCw}
          loading={syncing}
          disabled={blocked !== undefined}
          onClick={() => void sync()}
        >
          同步案例
        </Button>
        <Button variant="ghost" size="sm" leftIcon={Settings2} onClick={onEdit}>
          修改配置
        </Button>
      </div>
      {blocked ? <p className="text-xs text-ink-sub">{blocked}</p> : null}
      {syncError ? <p className="text-xs text-danger">{syncError}</p> : null}
    </li>
  )
}
