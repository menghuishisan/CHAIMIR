// 仿真实验室页(学生侧栏,/student/simulations)。
// 只做场景检索与版本选择:仿真会话由 M7 实验实例编排产生(POST /sim/sessions 在 internal 组),
// 学生不能独立建会话。因此本页能直接进入的只有「本机确定性推演」——
// 平台内置场景在浏览器 Worker 内按同一 seed 复现,不上报动作、不建实时连接(对齐清单 §6.6)。
//
// 本校自建场景(compute=isolated)在服务端隔离容器里运行,一个会话一个容器,
// 会话只能由课程实验编排产生,故本页对它不给「进入推演」入口 —— 不摊出必然失败的按钮。

import { useCallback, useMemo, useState } from 'react'
import { useNavigate } from 'react-router'
import { Layers, Network, Play, Search } from 'lucide-react'
import { SIM_COMPUTE, SIM_PACKAGE_STATUS, type SimPackageMeta } from '@chaimir/api-client'
import {
  Badge,
  Breadcrumb,
  Button,
  Callout,
  DataPanel,
  FilterBar,
  FilterField,
  FormField,
  Input,
  MetricStrip,
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
  Select,
  Skeleton,
  StatusIndicator,
  Table,
  type TableColumn,
} from '@chaimir/ui'
import { api } from '../../../../app/api'
import { ResourceState } from '../../../../components/ResourceState'
import { useAsyncResource, usePagedResource } from '../../../../hooks'
import { facetGroup, facetTopEntries } from '../../../../utils/facets'
import { formatDate } from '../../../../utils/formatters'
import {
  simCategoryLabel,
  simComputeLabel,
  simPackageStatusLabel,
} from '../../../../utils/labels/sim'
import { simPackageStatusTone } from '../../statusPresentation'

/**
 * StudentSimulationsPage 检索可用仿真场景并进入推演。
 */
export default function StudentSimulationsPage() {
  const [keyword, setKeyword] = useState('')
  const [submittedKeyword, setSubmittedKeyword] = useState('')
  const [versionPickerCode, setVersionPickerCode] = useState<string>()

  // 学生只看已发布场景:草稿与审核中的包属教师提交流程,下架包不再可用
  const packages = usePagedResource<SimPackageMeta>(
    (params) =>
      api.sim.getPackages({
        status: SIM_PACKAGE_STATUS.PUBLISHED,
        keyword: submittedKeyword || undefined,
        ...params,
      }),
    [submittedKeyword],
  )

  // 分类分布取后端 facets.category:与当前筛选同口径的全量分组计数,不用当前页切片去数(§6.5.4)
  const categoryKinds = Object.keys(facetGroup(packages.data?.facets, 'category')).length
  const topCategory = useMemo(
    () => facetTopEntries(packages.data?.facets, 'category', 1)[0],
    [packages.data],
  )

  // FilterBar 自己是 form 并接住回车,故这里只需要提交动作本身
  const handleSearch = useCallback(() => {
    setSubmittedKeyword(keyword.trim())
  }, [keyword])

  const columns: TableColumn<SimPackageMeta>[] = [
    {
      key: 'name',
      header: '仿真场景',
      render: (pkg) => (
        <div className="min-w-0">
          <div className="truncate font-medium text-ink">{pkg.name}</div>
          <div className="truncate text-xs text-ink-sub">{simCategoryLabel(pkg.category)}</div>
        </div>
      ),
    },
    {
      key: 'version',
      header: '当前版本',
      mono: true,
    },
    {
      key: 'compute',
      header: '运行位置',
      render: (pkg) => (
        <Badge tone={pkg.compute === SIM_COMPUTE.BROWSER ? 'jade' : 'info'}>
          {simComputeLabel(pkg.compute)}
        </Badge>
      ),
    },
    {
      key: 'updated_at',
      header: '更新时间',
      render: (pkg) => (
        <span className="whitespace-nowrap font-mono text-xs tabular-nums text-ink-sub">
          {formatDate(pkg.updated_at)}
        </span>
      ),
    },
    {
      key: 'status',
      header: '状态',
      render: (pkg) => (
        <StatusIndicator tone={simPackageStatusTone(pkg.status)} label={simPackageStatusLabel(pkg.status)} />
      ),
    },
    {
      key: 'actions',
      header: '操作',
      align: 'right',
      render: (pkg) =>
        pkg.compute === SIM_COMPUTE.BROWSER ? (
          <div className="flex justify-end gap-1">
            <Button variant="ghost" size="sm" onClick={() => setVersionPickerCode(pkg.code)}>
              选择版本
            </Button>
            <EnterSimulationButton code={pkg.code} version={pkg.version} />
          </div>
        ) : (
          <span className="block text-right text-xs text-ink-sub">随课程实验进入</span>
        ),
    },
  ]

  return (
    <PageScaffold>
      <PageHeader
        kicker={<Breadcrumb items={[{ label: '学习区' }]} />}
        title="仿真实验室"
        description="选一个场景进入推演。推演在你的浏览器里按固定随机种子运行,每次结果都可复现。"
        icon={Network}
      />

      {/* 指标退为一行内联摘要:分类分布走后端聚合契约(facets.category),
          是全量口径而非当前页切片(§6.5.4);算力位置在每一行里逐条可见。 */}
      <MetricStrip
        label="场景总量摘要"
        className="mb-5"
        items={[
          { label: '可用场景', value: packages.total, hint: '已发布给你的' },
          { label: '涵盖分类', value: categoryKinds, hint: '按当前搜索统计' },
          {
            label: '最多的分类',
            value: topCategory ? simCategoryLabel(topCategory.value) : '—',
            hint: topCategory ? `共 ${topCategory.count} 个场景` : '暂无场景',
          },
        ]}
      />

      <DataPanel
        label="场景列表"
        filter={
          <FilterBar label="场景筛选" onSubmit={handleSearch} submitLabel="搜索">
            <FilterField label="场景名称" htmlFor="sim-keyword">
              <Input
                id="sim-keyword"
                value={keyword}
                leftIcon={Search}
                placeholder="输入场景名称"
                onChange={(event) => setKeyword(event.target.value)}
              />
            </FilterField>
          </FilterBar>
        }
        footer={
          <Pagination
            page={packages.page}
            pageSize={packages.pageSize}
            total={packages.total}
            onPageChange={packages.setPage}
          />
        }
      >
          <ResourceState
            resource={packages}
            emptyIcon={Network}
            emptyTitle={submittedKeyword ? '没有匹配的场景' : '暂无可用场景'}
            emptyDescription={
              submittedKeyword
                ? '换个关键词再试,或清空搜索查看全部场景。'
                : '老师提交并通过审核的仿真场景会显示在这里。'
            }
            emptyAction={
              submittedKeyword ? (
                <Button
                  variant="outline"
                  onClick={() => {
                    setKeyword('')
                    setSubmittedKeyword('')
                  }}
                >
                  清空搜索
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
                // <md 换行卡(§6.4.1 规则 3):场景名一行、分类与版本一行
                mobileCard={(item) => ({
                  title: item.name,
                  meta: `${simCategoryLabel(item.category)} · 版本 ${item.version}`,
                })}
              />
            )}
          </ResourceState>
      </DataPanel>

      {versionPickerCode ? (
        <VersionPickerModal
          code={versionPickerCode}
          onClose={() => setVersionPickerCode(undefined)}
        />
      ) : null}
    </PageScaffold>
  )
}

interface EnterSimulationButtonProps {
  code: string
  version: string
}

/**
 * EnterSimulationButton 进入指定版本的推演工作台。
 * 版本随路由带入:同一场景的不同版本是不同的推演内容,刷新后必须还能回到同一版本。
 */
function EnterSimulationButton({ code, version }: EnterSimulationButtonProps) {
  const navigate = useNavigate()
  return (
    <Button
      variant="primary"
      size="sm"
      leftIcon={Play}
      onClick={() => navigate(`/student/simulations/${code}/workspace?version=${version}`)}
    >
      进入推演
    </Button>
  )
}

interface VersionPickerModalProps {
  code: string
  onClose: () => void
}

/**
 * VersionPickerModal 列出场景的全部版本供选择。
 * 老版本仍可进入:课程可能指定了特定版本作为教学材料。
 */
function VersionPickerModal({ code, onClose }: VersionPickerModalProps) {
  const navigate = useNavigate()
  const [selected, setSelected] = useState<string>()
  const versions = useAsyncResource(() => api.sim.getPackageVersions(code), [code])

  const options = useMemo(
    () =>
      (versions.data ?? [])
        .filter(
          (item) =>
            item.status === SIM_PACKAGE_STATUS.PUBLISHED && item.compute === SIM_COMPUTE.BROWSER,
        )
        .map((item) => ({ value: item.version, label: `${item.version} · ${formatDate(item.updated_at)}` })),
    [versions.data],
  )

  return (
    <Modal open onOpenChange={(open) => !open && onClose()}>
      <ModalContent size="sm">
        <ModalHeader>
          <ModalTitle>选择场景版本</ModalTitle>
          <ModalDescription>
            不同版本的推演内容可能不同。如果课程指定了版本,请按老师的要求选择。
          </ModalDescription>
        </ModalHeader>
        <ModalBody>
          <ResourceState
            resource={versions}
            emptyIcon={Layers}
            emptyTitle="暂无可用版本"
            emptyDescription="这个场景当前没有已发布的版本。"
            skeleton={<Skeleton variant="line" lines={2} />}
          >
            {() =>
              options.length === 0 ? (
                <Callout tone="info">这个场景当前没有已发布的版本。</Callout>
              ) : (
                <FormField label="场景版本" required>
                  <Select
                    options={options}
                    value={selected ?? options[0].value}
                    placeholder="选择版本"
                    onValueChange={setSelected}
                  />
                </FormField>
              )
            }
          </ResourceState>
        </ModalBody>
        <ModalFooter>
          <Button variant="outline" onClick={onClose}>
            取消
          </Button>
          <Button
            variant="primary"
            leftIcon={Play}
            disabled={options.length === 0}
            onClick={() =>
              navigate(
                `/student/simulations/${code}/workspace?version=${selected ?? options[0].value}`,
              )
            }
          >
            进入推演
          </Button>
        </ModalFooter>
      </ModalContent>
    </Modal>
  )
}
