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
  FormField,
  Input,
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
  Select,
  Skeleton,
  Stat,
  StatusIndicator,
  Table,
  type TableColumn,
} from '@chaimir/ui'
import { api } from '../../../../app/api'
import { ResourceState } from '../../../../components/ResourceState'
import { useAsyncResource, usePagedResource } from '../../../../hooks'
import { formatDate } from '../../../../utils/formatters'
import {
  simCategoryLabel,
  simComputeLabel,
  simPackageStatusLabel,
  simPackageStatusTone,
} from '../../../../utils/labels/sim'

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

  const handleSearch = useCallback((event: React.FormEvent<HTMLFormElement>) => {
    event.preventDefault()
    setSubmittedKeyword(keyword.trim())
  }, [keyword])

  const list = packages.data ? packages.data.list : []
  const categoryCount = new Set(list.map((item) => item.category)).size
  const browserCount = list.filter((item) => item.compute === SIM_COMPUTE.BROWSER).length

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
        kicker={<Breadcrumb items={[{ label: '学习区' }, { label: '仿真实验室' }]} />}
        title="仿真实验室"
        description="选一个场景进入推演。推演在你的浏览器里按固定随机种子运行,每次结果都可复现。"
        icon={Network}
      />

      <PageSection>
        <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-3">
          <Stat label="可用场景" value={packages.total} icon={Network} />
          <Stat label="涵盖分类" value={categoryCount} icon={Layers} />
          <Stat label="本机推演" value={browserCount} icon={Play} hint="无需服务端算力" />
        </div>
      </PageSection>

      <PageSection
        title="场景列表"
        description={`共 ${packages.total} 个可用场景`}
        actions={
          <form onSubmit={handleSearch} className="flex items-end gap-2">
            <FormField label="按名称搜索" className="mb-0">
              <Input
                value={keyword}
                leftIcon={Search}
                placeholder="输入场景名称"
                onChange={(event) => setKeyword(event.target.value)}
              />
            </FormField>
            <Button type="submit" variant="outline">
              搜索
            </Button>
          </form>
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
          skeleton={<Table columns={columns} data={[]} rowKey={() => ''} loading />}
        >
          {(page) => (
            <div className="flex flex-col gap-4">
              <Table columns={columns} data={page.list} rowKey={(item) => item.id} />
              <Pagination
                page={packages.page}
                pageSize={packages.pageSize}
                total={packages.total}
                onPageChange={packages.setPage}
              />
            </div>
          )}
        </ResourceState>
      </PageSection>

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
