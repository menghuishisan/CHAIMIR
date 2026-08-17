// 漏洞题工坊页(教师深页,/teacher/vuln-workshop)。
//
// 真实漏洞素材进题库的完整链路:配漏洞源 → 同步拉案例 → 预验证 → 固化到题库。
// 四步是同一条流水线上的相邻环节,拆成两个侧栏项会让教师在两页之间来回跳,
// 故一页两个分区(漏洞源 / 漏洞题草稿),对齐清单 §3.2 的两条深页在此合并呈现。
//
// 边界:同步、预验证、固化都是租户内的出题工作流(后端 teacher 组),
// 平台端只做全局漏洞源的查看与新增,不出现这三个动作。

import { useCallback, useMemo, useState } from 'react'
import {
  Bug,
  Database,
  FlaskConical,
  Plus,
  RefreshCw,
  Settings2,
  ShieldCheck,
} from 'lucide-react'
import {
  VulnProblemStatus,
  VulnPrevalidateStatus,
  type VulnProblem,
  type VulnSource,
} from '@chaimir/api-client'
import {
  Badge,
  Breadcrumb,
  Button,
  Callout,
  Card,
  CardBody,
  CardHeader,
  DescriptionList,
  Empty,
  FilterBar,
  FilterField,
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
  VULN_SOURCE_CONFIG_FIELDS,
  vulnLevelLabel,
  vulnLevelTone,
  vulnPrevalidateStatusLabel,
  vulnPrevalidateStatusTone,
  vulnProblemStatusLabel,
  vulnProblemStatusTone,
  vulnRuntimeModeLabel,
  vulnSourceTypeLabel,
} from '../../../../utils/labels/contest'
import { userFacingErrorMessage } from '../../../../utils/userFacingError'
import { VulnSourceFormModal } from '../vuln-source-form'
import { VulnProblemFormModal } from './vuln-problem-form'
import { VulnPrevalidateModal } from './vuln-prevalidate'

/** 状态筛选项:值为空串表示不过滤。 */
const STATUS_FILTERS = [
  { value: '', label: '全部' },
  { value: String(VulnProblemStatus.DRAFT), label: '草稿' },
  { value: String(VulnProblemStatus.FINALIZED), label: '已固化' },
  { value: String(VulnProblemStatus.DISCARDED), label: '已弃用' },
] as const

/**
 * TeacherVulnWorkshopPage 承载漏洞源维护与漏洞题转化。
 */
export default function TeacherVulnWorkshopPage() {
  const [sourceFilter, setSourceFilter] = useState<string>('')
  const [statusFilter, setStatusFilter] = useState<string>('')
  const [sourceForm, setSourceForm] = useState<{ source?: VulnSource } | undefined>()
  const [problemForm, setProblemForm] = useState<{ sources: VulnSource[] } | undefined>()
  const [prevalidateTarget, setPrevalidateTarget] = useState<VulnProblem>()
  const [finalizingId, setFinalizingId] = useState<string>()
  const [actionError, setActionError] = useState<string>()

  const sources = useAsyncResource(() => api.contest.listVulnSources(), [], () => false)

  const problems = usePagedResource<VulnProblem>(
    (params) =>
      api.contest.listVulnProblems({
        source_id: sourceFilter || undefined,
        status: statusFilter ? (Number(statusFilter) as VulnProblemStatus) : undefined,
        ...params,
      }),
    [sourceFilter, statusFilter],
  )

  const sourceList = useMemo(() => sources.data ?? [], [sources.data])

  const sourceNameById = useMemo(
    () => new Map(sourceList.map((source) => [source.id, source.name])),
    [sourceList],
  )

  /** finalize 把预验证通过的草稿固化到题库,固化后即可作为赛题引用。 */
  const finalize = useCallback(
    async (problem: VulnProblem) => {
      setFinalizingId(problem.id)
      setActionError(undefined)
      try {
        await api.contest.finalizeVulnProblem(problem.id)
        toast.success('漏洞题已固化到题库')
        problems.reload()
      } catch (error) {
        setActionError(userFacingErrorMessage(error, '固化没有成功,请稍后重试。'))
      } finally {
        setFinalizingId(undefined)
      }
    },
    [problems],
  )

  // 指标带取服务端全量口径,不随下方筛选变化。
  // 预验证状态没有服务端筛选参数,故不做「验证通过/尚未验证」两张卡:
  // 用当前页数出来的数字在总量更大时是错数,验证状态本身在表格里逐条可见(规范 §6.5)。
  const totalCount = useResourceTotal((params) => api.contest.listVulnProblems(params), [])
  const draftCount = useResourceTotal(
    (params) => api.contest.listVulnProblems({ status: VulnProblemStatus.DRAFT, ...params }),
    [],
  )
  const finalizedCount = useResourceTotal(
    (params) => api.contest.listVulnProblems({ status: VulnProblemStatus.FINALIZED, ...params }),
    [],
  )

  const columns: TableColumn<VulnProblem>[] = [
    {
      key: 'title',
      header: '漏洞题',
      render: (problem) => (
        <div className="min-w-0">
          <div className="truncate font-medium text-ink">{problem.title}</div>
          <div className="truncate text-xs text-ink-sub">
            {problem.source_id ? (sourceNameById.get(problem.source_id) ?? '已删除的来源') : '手工录入'}
            {problem.external_ref ? ` · ${problem.external_ref}` : ''}
          </div>
        </div>
      ),
    },
    {
      key: 'level',
      header: '可复现性',
      render: (problem) => (
        <Badge tone={vulnLevelTone(problem.level)}>{vulnLevelLabel(problem.level)}</Badge>
      ),
    },
    {
      key: 'runtime_mode',
      header: '复现方式',
      render: (problem) => (
        <span className="text-sm text-ink-sub">{vulnRuntimeModeLabel(problem.runtime_mode)}</span>
      ),
    },
    {
      key: 'prevalidate_status',
      header: '预验证',
      render: (problem) => (
        <StatusIndicator
          tone={vulnPrevalidateStatusTone(problem.prevalidate_status)}
          label={vulnPrevalidateStatusLabel(problem.prevalidate_status)}
        />
      ),
    },
    {
      key: 'status',
      header: '状态',
      render: (problem) => (
        <div className="flex flex-wrap items-center gap-1.5">
          <StatusIndicator
            tone={vulnProblemStatusTone(problem.status)}
            label={vulnProblemStatusLabel(problem.status)}
          />
          {problem.content_item_code ? (
            <Badge tone="jade">题库 {problem.content_item_code}</Badge>
          ) : null}
        </div>
      ),
    },
    {
      key: 'actions',
      header: '操作',
      align: 'right',
      render: (problem) => (
        <div className="flex items-center justify-end gap-1">
          <Button
            variant="ghost"
            size="sm"
            leftIcon={FlaskConical}
            onClick={() => setPrevalidateTarget(problem)}
          >
            预验证
          </Button>
          {problem.prevalidate_status === VulnPrevalidateStatus.PASSED &&
          problem.status === VulnProblemStatus.DRAFT ? (
            <Button
              variant="ghost"
              size="sm"
              leftIcon={ShieldCheck}
              loading={finalizingId === problem.id}
              onClick={() => void finalize(problem)}
            >
              固化到题库
            </Button>
          ) : null}
        </div>
      ),
    },
  ]

  return (
    <PageScaffold>
      <PageHeader
        kicker={
          <Breadcrumb
            items={[
              { label: '实践' },
              { label: '赛事组织', href: '/teacher/contests' },
              { label: '漏洞题工坊' },
            ]}
          />
        }
        title="漏洞题工坊"
        description="把真实漏洞素材变成可用赛题:配置来源、同步案例、预验证可复现,最后固化进题库。"
        icon={Bug}
        actions={
          <Button
            variant="primary"
            leftIcon={Plus}
            onClick={() => setProblemForm({ sources: sourceList })}
          >
            手工录入漏洞题
          </Button>
        }
      />

      <PageSection>
        <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-3">
          <Stat label="漏洞题总数" value={totalCount ?? '—'} icon={Bug} hint="不受下方筛选影响" />
          <Stat label="草稿" value={draftCount ?? '—'} icon={FlaskConical} hint="预验证通过后可固化" />
          <Stat label="已固化" value={finalizedCount ?? '—'} icon={Database} hint="可作为赛题引用" />
        </div>
      </PageSection>

      <PageSection
        title="漏洞来源"
        description="从公开漏洞情报或弱点分类库拉取案例。密钥类配置由后端加密保存,不回显。"
        actions={
          <Button variant="outline" leftIcon={Plus} onClick={() => setSourceForm({})}>
            添加来源
          </Button>
        }
      >
        <ResourceState
          resource={sources}
          emptyIcon={Database}
          emptyTitle="还没有配置漏洞来源"
          emptyDescription="配置来源后可以同步拉取漏洞案例,也可以先手工录入单个漏洞题。"
          emptyAction={
            <Button variant="outline" leftIcon={Plus} onClick={() => setSourceForm({})}>
              添加来源
            </Button>
          }
        >
          {(list) => (
            <div className="grid gap-4 lg:grid-cols-2">
              {list.map((source) => (
                <VulnSourceCard
                  key={source.id}
                  source={source}
                  onEdit={() => setSourceForm({ source })}
                  onSynced={() => {
                    sources.reload()
                    problems.reload()
                  }}
                  onError={setActionError}
                />
              ))}
            </div>
          )}
        </ResourceState>
      </PageSection>

      <PageSection
        title="漏洞题草稿"
        description={`共 ${problems.total} 条草稿。验证通过后固化进题库,才能作为赛题引用。`}
      >
        <div className="flex flex-col gap-4">
          <FilterBar label="漏洞题草稿筛选">
            <FilterField label="草稿状态" group>
              <SegmentedControl
                aria-label="按草稿状态筛选"
                size="sm"
                options={STATUS_FILTERS.map((item) => ({ value: item.value, label: item.label }))}
                value={statusFilter}
                onValueChange={setStatusFilter}
              />
            </FilterField>
            <FilterField label="来源" htmlFor="vuln-source-filter">
              <Select
                id="vuln-source-filter"
                options={[
                  { value: '', label: '全部来源' },
                  ...sourceList.map((source) => ({ value: source.id, label: source.name })),
                ]}
                value={sourceFilter}
                placeholder="全部来源"
                onValueChange={setSourceFilter}
              />
            </FilterField>
          </FilterBar>
          {actionError ? <Callout tone="danger">{actionError}</Callout> : null}

          <ResourceState
            resource={problems}
            emptyIcon={Bug}
            emptyTitle={statusFilter || sourceFilter ? '这个条件下没有草稿' : '还没有漏洞题草稿'}
            emptyDescription={
              statusFilter || sourceFilter
                ? '换个条件再看,或清空筛选查看全部草稿。'
                : '从来源同步案例,或手工录入一条漏洞题。'
            }
            skeleton={<Table columns={columns} data={[]} rowKey={() => ''} loading />}
          >
            {(page) => (
              <>
                <Table columns={columns} data={page.list} rowKey={(item) => item.id} />
                <Pagination
                  page={problems.page}
                  pageSize={problems.pageSize}
                  total={problems.total}
                  onPageChange={problems.setPage}
                />
              </>
            )}
          </ResourceState>
        </div>
      </PageSection>

      <Callout tone="info">
        固化会把漏洞题写入本校题库并自动发布,答案与判题配置在写入时即被标记为敏感字段,学生取题面时看不到。
      </Callout>

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

      {problemForm ? (
        <VulnProblemFormModal
          sources={problemForm.sources}
          onClose={() => setProblemForm(undefined)}
          onSaved={() => {
            setProblemForm(undefined)
            problems.reload()
          }}
        />
      ) : null}

      {prevalidateTarget ? (
        <VulnPrevalidateModal
          problem={prevalidateTarget}
          onClose={() => setPrevalidateTarget(undefined)}
          onDone={() => {
            setPrevalidateTarget(undefined)
            problems.reload()
          }}
        />
      ) : null}
    </PageScaffold>
  )
}

interface VulnSourceCardProps {
  source: VulnSource
  onEdit: () => void
  onSynced: () => void
  onError: (message: string) => void
}

/**
 * VulnSourceCard 展示单个漏洞来源并承载同步。
 * 同步是拉取外部数据的动作,可能耗时,故按钮带 loading 并在完成后提示落库条数。
 */
function VulnSourceCard({ source, onEdit, onSynced, onError }: VulnSourceCardProps) {
  const [syncing, setSyncing] = useState(false)

  const sync = useCallback(async () => {
    setSyncing(true)
    try {
      const created = await api.contest.syncVulnSource(source.id)
      toast.success(
        created.length > 0 ? `已同步 ${created.length} 条漏洞案例` : '同步完成,来源里没有新案例',
      )
      onSynced()
    } catch (error) {
      onError(userFacingErrorMessage(error, '同步没有成功,请检查来源配置后重试。'))
    } finally {
      setSyncing(false)
    }
  }, [onError, onSynced, source.id])

  const endpoint = readString(source.config, VULN_SOURCE_CONFIG_FIELDS.endpoint)

  return (
    <Card>
      <CardHeader
        title={source.name}
        description={vulnSourceTypeLabel(source.type)}
        actions={
          source.enabled ? <Badge tone="success">已启用</Badge> : <Badge tone="neutral">已停用</Badge>
        }
      />
      <CardBody className="flex flex-col gap-3">
        <DescriptionList
          dense
          items={[
            { term: '默认分级', description: vulnLevelLabel(source.default_level) },
            {
              term: '来源地址',
              description: endpoint || '未配置',
              mono: true,
            },
            {
              term: '上次同步',
              description: source.last_sync_at ? formatDateTime(source.last_sync_at) : '尚未同步',
              mono: true,
            },
          ]}
        />
        <div className="flex flex-wrap items-center gap-2">
          <Button
            variant="outline"
            size="sm"
            leftIcon={RefreshCw}
            loading={syncing}
            disabled={!source.enabled}
            onClick={() => void sync()}
          >
            同步案例
          </Button>
          <Button variant="ghost" size="sm" leftIcon={Settings2} onClick={onEdit}>
            修改配置
          </Button>
        </div>
        {source.enabled ? null : (
          <Empty
            icon={Database}
            title="来源已停用"
            description="启用后才能同步案例。"
          />
        )}
      </CardBody>
    </Card>
  )
}

/** readString 从来源配置里读字符串字段;被后端脱敏的字段回空串,不把掩码对象抛到界面上。 */
function readString(source: Record<string, unknown>, key: string): string {
  const value = source[key]
  return typeof value === 'string' ? value : ''
}
